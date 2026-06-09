package main

import (
	"context"
	"crypto/tls"

	"github.com/tsaarni/echoclient/worker"
)

func runTLS(cmd TLSCmd) {
	httpClient := newHTTPClient(&tls.Config{InsecureSkipVerify: true})
	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		return doGet(ctx, &httpClient, "https://echoserver-tls.127.0.0.1.nip.io/")
	})
}
