package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
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

type workerFlags struct {
	RPS         int           `help:"Requests per second." default:"1000"`
	Concurrency int           `help:"Parallel workers." default:"50"`
	Count       int           `help:"Number of requests, 0=infinite." default:"0"`
	Duration    time.Duration `help:"Run duration (0=use count)." default:"60s"`
}

type HTTPCmd struct {
	workerFlags
}

type TLSCmd struct {
	workerFlags
}

type CertAuthCmd struct {
	Cert string `help:"Client certificate PEM file." default:"../certs/external-client.pem"`
	Key  string `help:"Client key PEM file." default:"../certs/external-client-key.pem"`
	workerFlags
}

type JWTLoadCmd struct {
	SigningKey string `help:"JWT signing key PEM file." default:"../certs/jwt-signing-key.pem"`
	Issuer    string `help:"Token issuer." default:"https://example.com"`
	Audience  string `help:"Token audience." default:"echoserver"`
	Subject   string `help:"Token subject." default:"user"`
	workerFlags
}

type UploadCmd struct {
	Size int `help:"Upload body size in bytes." default:"1048576"`
	workerFlags
}

type ConnectionsCmd struct {
	workerFlags
}

type StatsCmd struct{}

var cli struct {
	Verbose bool `help:"Enable debug logging." short:"v"`

	HTTP        HTTPCmd        `cmd:"http" help:"Plain HTTP traffic to echoserver."`
	TLS         TLSCmd         `cmd:"tls" help:"HTTPS traffic to echoserver-tls."`
	CertAuth    CertAuthCmd    `cmd:"cert-auth" help:"HTTPS + client cert traffic."`
	JWTLoad     JWTLoadCmd     `cmd:"jwt-load" help:"HTTPS + JWT token traffic."`
	Upload      UploadCmd      `cmd:"upload" help:"POST large bodies to echoserver."`
	Connections ConnectionsCmd `cmd:"connections" help:"High connection churn (no keep-alive)."`
	Stats       StatsCmd       `cmd:"stats" help:"Show cluster resource stats."`
	JWKS        JWKSCmd        `cmd:"jwks" help:"Manage JWKS key material."`
	JWT         JWTCmd         `cmd:"jwt" help:"Generate JWT tokens."`
}

func main() {
	ctx := kong.Parse(&cli, kong.UsageOnError())
	switch ctx.Command() {
	case "stats":
		runStats()
	case "jwks generate":
		runJWKSGenerate(&cli.JWKS.Generate)
	case "jwt sign":
		runJWTSign(&cli.JWT.Sign)
	case "http":
		withMetrics(func() { runHTTP(cli.HTTP) })
	case "tls":
		withMetrics(func() { runTLS(cli.TLS) })
	case "cert-auth":
		withMetrics(func() { runCertAuth(cli.CertAuth) })
	case "jwt-load":
		withMetrics(func() { runJWTLoad(cli.JWTLoad) })
	case "upload":
		withMetrics(func() { runUpload(cli.Upload) })
	case "connections":
		withMetrics(func() { runConnections(cli.Connections) })
	default:
		ctx.FatalIfErrorf(ctx.PrintUsage(false))
		os.Exit(1)
	}
}

// withMetrics sets up signal handling and periodic metric dumps around a traffic function.
func withMetrics(f func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nInterrupted. Dumping metrics:")
		metrics.DumpMetrics(os.Stdout)
		os.Exit(1)
	}()

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

	f()

	ticker.Stop()
	close(done)
	<-stopped
	metrics.DumpMetrics(os.Stdout)
}

// run creates and launches a worker pool with the given flags and worker function.
func run(flags workerFlags, f worker.WorkerFunc) {
	opts := []worker.Option{
		worker.WithConcurrency(flags.Concurrency),
		worker.WithRateLimit(flags.RPS, flags.RPS),
	}
	if flags.Duration > 0 {
		opts = append(opts, worker.WithDuration(flags.Duration))
	} else if flags.Count > 0 {
		opts = append(opts, worker.WithRepetitions(flags.Count))
	} else {
		opts = append(opts, worker.WithInfiniteRepetitions())
	}

	pool := worker.NewWorkerPool(f, opts...)
	if err := pool.Launch(); err != nil {
		log.Fatal(err)
	}
	pool.Wait()
}

// newHTTPClient creates an http.Client with MeasuringRoundTripper and the given TLS config.
func newHTTPClient(tlsCfg *tls.Config) http.Client {
	return http.Client{
		Transport: client.NewMeasuringRoundTripper(&http.Transport{
			TLSClientConfig:     tlsCfg,
			MaxConnsPerHost:     10000,
			MaxIdleConns:        10000,
			MaxIdleConnsPerHost: 10000,
			ForceAttemptHTTP2:   true,
		}),
	}
}

// doGet performs an HTTP GET and discards the body.
func doGet(ctx context.Context, httpClient *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// doGetWithHeaders performs an HTTP GET with custom headers and discards the body.
func doGetWithHeaders(ctx context.Context, httpClient *http.Client, url string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// doPost performs an HTTP POST with the given body and discards the response.
func doPost(ctx context.Context, httpClient *http.Client, url string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}
