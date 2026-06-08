package main

import (
	"context"

	"github.com/tsaarni/echoclient/worker"
)

func runHTTP(cmd HTTPCmd) {
	httpClient := newHTTPClient(nil)
	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		return doGet(ctx, &httpClient, "http://echoserver.127.0.0.1.nip.io/")
	})
}
