package main

import (
	"context"
	"net/http"

	"github.com/tsaarni/echoclient/client"
	"github.com/tsaarni/echoclient/worker"
)

func runConnections(cmd ConnectionsCmd) {
	// Disable keep-alive to force new TCP connection per request.
	httpClient := http.Client{
		Transport: client.NewMeasuringRoundTripper(&http.Transport{
			MaxConnsPerHost:     10000,
			MaxIdleConns:        0,
			MaxIdleConnsPerHost: 0,
			DisableKeepAlives:   true,
		}),
	}
	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		return doGet(ctx, &httpClient, "http://echoserver.127.0.0.1.nip.io/")
	})
}
