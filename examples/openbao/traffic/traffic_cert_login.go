package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/client"
	"github.com/tsaarni/echoclient/worker"
)

func runCertLogin(addr string, tlsCfg *tls.Config, tokenType string, count, rps, concurrency int) {
	slog.Info("Starting cert-login", "count", humanize.Comma(int64(count)), "token-type", tokenType)

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

	body := []byte(fmt.Sprintf(`{"name":"%s"}`, tokenType))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		req, _ := http.NewRequestWithContext(ctx, "POST", addr+"/v1/auth/cert/login", bytes.NewReader(body))
		resp, err := loginClient.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	})
}
