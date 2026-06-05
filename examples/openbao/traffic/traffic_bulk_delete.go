package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/worker"
)

func runBulkDelete(addr, token string, count, rps, concurrency int) {
	slog.Info("Starting bulk-delete", "count", humanize.Comma(int64(count)))
	var counter atomic.Int64
	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		i := counter.Add(1)
		resp, err := doReqCtx(ctx, "DELETE", fmt.Sprintf("%s/v1/secret/key-%d", addr, i), token, nil)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	})
}
