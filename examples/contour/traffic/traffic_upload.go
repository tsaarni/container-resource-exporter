package main

import (
	"context"

	"github.com/tsaarni/echoclient/generator"
	"github.com/tsaarni/echoclient/worker"
)

func runUpload(cmd UploadCmd) {
	httpClient := newHTTPClient(nil)
	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		body := generator.NewReader(generator.WithASCII(), generator.WithTotalSize(uint64(cmd.Size)))
		return doPost(ctx, &httpClient, "http://echoserver.127.0.0.1.nip.io/", body)
	})
}
