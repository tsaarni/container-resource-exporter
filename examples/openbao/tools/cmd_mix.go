package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/client"
	"github.com/tsaarni/echoclient/worker"
)

var validMixOps = map[string]bool{"read": true, "write": true, "delete": true, "transit": true, "login": true}

func parseMixWeights(s string) map[string]int {
	m := make(map[string]int)
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			log.Fatalf("invalid mix format: %q", part)
		}
		if !validMixOps[kv[0]] {
			log.Fatalf("unknown mix operation %q (valid: read, write, delete, transit, login)", kv[0])
		}
		var v int
		if _, err := fmt.Sscanf(kv[1], "%d", &v); err != nil {
			log.Fatalf("invalid mix value: %q", part)
		}
		m[kv[0]] = v
	}
	return m
}

func runMix(addr string, tlsCfg *tls.Config, token, engine, mixStr, tokenType string, rps, concurrency, size, pool int, duration time.Duration) {
	weights := parseMixWeights(mixStr)

	// Maintains a pool of keys with churn (create new + delete random).
	var listMount, dataPrefix, deletePrefix string
	if engine == "2" {
		listMount = "kv2/metadata"
		dataPrefix = "/v1/kv2/data/"
		deletePrefix = "/v1/kv2/metadata/"
	} else {
		listMount = "kv1"
		dataPrefix = "/v1/kv1/"
		deletePrefix = "/v1/kv1/"
	}
	existing := listKeys(addr, token, listMount)

	if len(existing) < pool {
		slog.Debug("Seeding pool", "engine", engine, "need", humanize.Comma(int64(pool-len(existing))))
		var payload []byte
		if engine == "2" {
			payload = generateKV2Payload(size)
		} else {
			payload = generateKV1Payload(size)
		}
		var counter atomic.Int64
		counter.Store(int64(len(existing)))
		run(pool-len(existing), rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
			i := counter.Add(1)
			return post(ctx, fmt.Sprintf("%s%skey-%d", addr, dataPrefix, i), token, payload)
		})
		existing = listKeys(addr, token, listMount)
	}

	slog.Info("Starting mix", "duration", duration, "pool", humanize.Comma(int64(pool)), "mix", mixStr)

	// Prepare cert login client
	certFile := filepath.Join("..", "certs", "client.pem")
	keyFile := filepath.Join("..", "certs", "client-key.pem")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Load client cert: %v", err)
	}
	loginClient := &http.Client{Transport: client.NewMeasuringRoundTripper(&http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      tlsCfg.RootCAs,
			Certificates: []tls.Certificate{cert},
		},
	})}

	plaintext := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("A", size)))
	encPayload := []byte(fmt.Sprintf(`{"plaintext":"%s"}`, plaintext))
	var writePayload []byte
	if engine == "2" {
		writePayload = generateKV2Payload(size)
	} else {
		writePayload = generateKV1Payload(size)
	}

	// Reservoir: track live keys for random read/delete
	var mu sync.Mutex
	keys := make([]string, len(existing))
	copy(keys, existing)
	var nextID atomic.Int64
	nextID.Store(int64(len(existing)))

	var readFunc worker.WorkerFunc = func(ctx context.Context, _ *worker.WorkerPool) error {
		mu.Lock()
		if len(keys) == 0 {
			mu.Unlock()
			return nil
		}
		key := keys[rand.IntN(len(keys))]
		mu.Unlock()
		return get(ctx, fmt.Sprintf("%s%s%s", addr, dataPrefix, key), token)
	}

	var writeFunc worker.WorkerFunc = func(ctx context.Context, _ *worker.WorkerPool) error {
		id := nextID.Add(1)
		key := fmt.Sprintf("key-%d", id)
		if err := post(ctx, fmt.Sprintf("%s%s%s", addr, dataPrefix, key), token, writePayload); err != nil {
			return err
		}
		mu.Lock()
		keys = append(keys, key)
		mu.Unlock()
		return nil
	}

	var deleteFunc worker.WorkerFunc = func(ctx context.Context, _ *worker.WorkerPool) error {
		mu.Lock()
		if len(keys) == 0 {
			mu.Unlock()
			return nil
		}
		idx := rand.IntN(len(keys))
		key := keys[idx]
		keys[idx] = keys[len(keys)-1]
		keys = keys[:len(keys)-1]
		mu.Unlock()
		resp, err := doReqCtx(ctx, "DELETE", fmt.Sprintf("%s%s%s", addr, deletePrefix, key), token, nil)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	}

	var transitFunc worker.WorkerFunc = func(ctx context.Context, _ *worker.WorkerPool) error {
		resp, err := doReqCtx(ctx, "POST", addr+"/v1/transit/encrypt/default", token, encPayload)
		if err != nil {
			return err
		}
		var encResp struct {
			Data struct {
				Ciphertext string `json:"ciphertext"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&encResp)
		resp.Body.Close()
		decPayload := []byte(fmt.Sprintf(`{"ciphertext":"%s"}`, encResp.Data.Ciphertext))
		return post(ctx, addr+"/v1/transit/decrypt/default", token, decPayload)
	}

	loginBody := []byte(fmt.Sprintf(`{"name":"%s"}`, tokenType))

	var loginFunc worker.WorkerFunc = func(ctx context.Context, _ *worker.WorkerPool) error {
		req, err := http.NewRequestWithContext(ctx, "POST", addr+"/v1/auth/cert/login", bytes.NewReader(loginBody))
		if err != nil {
			return err
		}
		resp, err := loginClient.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	}

	composedWorker := worker.Mix(
		readFunc.Weighted(weights["read"]),
		writeFunc.Weighted(weights["write"]),
		deleteFunc.Weighted(weights["delete"]),
		transitFunc.Weighted(weights["transit"]),
		loginFunc.Weighted(weights["login"]),
	)

	opts := []worker.Option{
		worker.WithConcurrency(concurrency),
		worker.WithRateLimit(rps, rps),
	}
	if duration > 0 {
		opts = append(opts, worker.WithDuration(duration))
	} else {
		opts = append(opts, worker.WithInfiniteRepetitions())
	}

	p := worker.NewWorkerPool(composedWorker, opts...)
	if err := p.Launch(); err != nil {
		log.Fatal(err)
	}
	p.Wait()
}


