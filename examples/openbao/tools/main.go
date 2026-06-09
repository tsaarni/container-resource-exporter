package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/tsaarni/echoclient/client"
	"github.com/tsaarni/echoclient/metrics"
	"github.com/tsaarni/echoclient/worker"
)

var httpClient http.Client

type workerFlags struct {
	RPS         int `help:"Requests per second." default:"200"`
	Concurrency int `help:"Parallel workers." default:"10"`
}

type commonFlags struct {
	Token string `help:"OpenBao token." env:"BAO_TOKEN" required:""`
	Count int    `help:"Number of requests, 0=infinite." default:"1000"`
	workerFlags
}

type KVWriteCmd struct {
	commonFlags
	KVEngine string `help:"KV engine version." default:"1" enum:"1,2"`
	Size     int    `help:"Payload size in bytes." default:"10240"`
}

type KVReadCmd struct {
	commonFlags
	KVEngine string `help:"KV engine version." default:"1" enum:"1,2"`
	Pool     int    `help:"Secret pool size." default:"500"`
}

type TransitCmd struct {
	commonFlags
	Size int `help:"Payload size in bytes." default:"10240"`
}

type CertLoginCmd struct {
	TokenType string `help:"Token type: batch or service." default:"batch" enum:"batch,service"`
	Count     int    `help:"Number of requests, 0=infinite." default:"1000"`
	workerFlags
}

type MixCmd struct {
	Token       string        `help:"OpenBao token." env:"BAO_TOKEN" required:""`
	KVEngine    string        `help:"KV engine version." default:"2" enum:"1,2"`
	Mix         string        `help:"Traffic mix as relative weights (e.g. read=7,write=1,delete=1,transit=1)." default:"read=70,write=10,delete=10,transit=5,login=5"`
	TokenType   string        `help:"Token type for cert login: batch or service." default:"batch" enum:"batch,service"`
	Size        int           `help:"Payload size in bytes." default:"10240"`
	Pool        int           `help:"Secret pool size." default:"500"`
	Duration    time.Duration `help:"Run duration (e.g. 5m, 0=infinite)." default:"0"`
	workerFlags
}

type BulkDeleteCmd struct {
	Token string `help:"OpenBao token." env:"BAO_TOKEN" required:""`
	workerFlags
}

type StatsCmd struct{}

var cli struct {
	Addr    string `help:"OpenBao address." env:"BAO_ADDR" default:"https://localhost:8200" hidden:""`
	CACert  string `help:"Path to CA certificate." env:"BAO_CACERT" default:"../certs/ca.pem"`
	Verbose bool   `help:"Enable debug logging." short:"v"`

	KVWrite    KVWriteCmd    `cmd:"kv-write" help:"Write secrets to a KV engine (builds stored data)."`
	KVRead     KVReadCmd     `cmd:"kv-read" help:"Read random secrets from a KV engine."`
	Transit    TransitCmd    `cmd:"transit" help:"Encrypt/decrypt via transit engine."`
	CertLogin  CertLoginCmd  `cmd:"cert-login" help:"Login using TLS client certificate."`
	Mix        MixCmd        `cmd:"mix" help:"Churn on a KV engine with configurable traffic mix."`
	BulkDelete BulkDeleteCmd `cmd:"bulk-delete" help:"Delete all secrets from kv1/ and kv2/ (cleanup helper)."`
	Stats      StatsCmd      `cmd:"stats" help:"Show cluster resource stats."`
}

func loadCACert(path string) *tls.Config {
	ca, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read CA cert: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		log.Fatalf("failed to parse CA cert: %s", path)
	}
	return &tls.Config{RootCAs: pool}
}

func main() {
	ctx := kong.Parse(&cli, kong.UsageOnError())

	if cli.Verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}
	addr := cli.Addr
	tlsCfg := loadCACert(cli.CACert)

	httpClient = http.Client{
		Timeout: 30 * time.Second,
		Transport: client.NewMeasuringRoundTripper(&http.Transport{
			TLSClientConfig:     tlsCfg,
			MaxConnsPerHost:     10000,
			MaxIdleConns:        10000,
			MaxIdleConnsPerHost: 10000,
			ForceAttemptHTTP2:   true,
		}),
	}

	if ctx.Command() == "stats" {
		runStats()
		return
	}

	// Print metrics on interrupt and exit.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nInterrupted. Dumping metrics:")
		metrics.DumpMetrics(os.Stdout)
		os.Exit(1)
	}()

	// Print metrics every 5 seconds.
	ticker := time.NewTicker(5 * time.Second)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-ticker.C:
				metrics.DumpMetrics(os.Stdout)
			case <-done:
				return
			}
		}
	}()

	switch ctx.Command() {
	case "kv-write":
		runKVWrite(addr, cli.KVWrite.Token, cli.KVWrite.KVEngine, cli.KVWrite.Count, cli.KVWrite.RPS, cli.KVWrite.Concurrency, cli.KVWrite.Size)
	case "kv-read":
		runKVRead(addr, cli.KVRead.Token, cli.KVRead.KVEngine, cli.KVRead.Count, cli.KVRead.RPS, cli.KVRead.Concurrency, cli.KVRead.Pool)
	case "transit":
		runTransit(addr, cli.Transit.Token, cli.Transit.Count, cli.Transit.RPS, cli.Transit.Concurrency, cli.Transit.Size)
	case "cert-login":
		runCertLogin(addr, tlsCfg, cli.CertLogin.TokenType, cli.CertLogin.Count, cli.CertLogin.RPS, cli.CertLogin.Concurrency)
	case "mix":
		runMix(addr, tlsCfg, cli.Mix.Token, cli.Mix.KVEngine, cli.Mix.Mix, cli.Mix.TokenType, cli.Mix.RPS, cli.Mix.Concurrency, cli.Mix.Size, cli.Mix.Pool, cli.Mix.Duration)
	case "bulk-delete":
		runBulkDelete(addr, cli.BulkDelete.Token, cli.BulkDelete.RPS, cli.BulkDelete.Concurrency)
	}

	ticker.Stop()
	close(done)
	<-stopped
	metrics.DumpMetrics(os.Stdout)
}

// --- Helpers ---

func get(ctx context.Context, url, token string) error {
	resp, err := doReqCtx(ctx, "GET", url, token, nil)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return nil
}

func post(ctx context.Context, url, token string, body []byte) error {
	resp, err := doReqCtx(ctx, "POST", url, token, body)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("POST %s: status %d", url, resp.StatusCode)
	}
	return nil
}

func listKeys(addr, token, mount string) []string {
	req, _ := http.NewRequest("GET", addr+"/v1/"+mount+"?list=true", nil)
	req.Header.Set("X-Vault-Token", token)
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()
	var result struct {
		Data struct{ Keys []string `json:"keys"` } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data.Keys
}

func listKeysPage(addr, token, mount string, after string, limit int) []string {
	url := fmt.Sprintf("%s/v1/%s?list=true&limit=%d", addr, mount, limit)
	if after != "" {
		url += "&after=" + after
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Vault-Token", token)
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()
	var result struct {
		Data struct{ Keys []string `json:"keys"` } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data.Keys
}

func getKeyCount(addr, token, mount string) int {
	return len(listKeys(addr, token, mount))
}

func run(count, rps, concurrency int, f worker.WorkerFunc) {
	opts := []worker.Option{
		worker.WithConcurrency(concurrency),
		worker.WithRateLimit(rps, rps),
	}
	if count > 0 {
		opts = append(opts, worker.WithRepetitions(count))
	} else {
		opts = append(opts, worker.WithInfiniteRepetitions())
	}

	pool := worker.NewWorkerPool(f, opts...)
	if err := pool.Launch(); err != nil {
		log.Fatal(err)
	}
	pool.Wait()
}

func doReqCtx(ctx context.Context, method, url, token string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", token)
	return httpClient.Do(req)
}
