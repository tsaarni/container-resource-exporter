package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/client"
	"github.com/tsaarni/echoclient/worker"
)

func runCertLogin(addr string, tlsCfg *tls.Config, count, rps, concurrency int) {
	slog.Info("Starting cert-login", "count", humanize.Comma(int64(count)))

	certFile := certPath("client.pem")
	keyFile := certPath("client-key.pem")
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

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		req, _ := http.NewRequestWithContext(ctx, "POST", addr+"/v1/auth/cert/login", nil)
		resp, err := loginClient.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	})
}

func certPath(name string) string {
	exe, _ := os.Executable()
	p := fmt.Sprintf("%s/certs/%s", strings.TrimSuffix(exe, "/traffic"), name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "certs/" + name
}
