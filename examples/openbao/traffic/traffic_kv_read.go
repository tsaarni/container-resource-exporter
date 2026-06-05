package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand/v2"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/worker"
)

func runKVRead(addr, token string, count, rps, concurrency, pool int) {
	// Uses KV v1 at secret/
	existing := getKeyCount(addr, token, "secret")
	if existing == 0 {
		log.Fatal("No secrets to read. Run kv-write first.")
	}
	if pool > existing {
		pool = existing
	}
	slog.Info("Starting kv-read", "count", humanize.Comma(int64(count)), "pool", humanize.Comma(int64(pool)))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		i := rand.IntN(pool) + 1
		return get(ctx, fmt.Sprintf("%s/v1/secret/key-%d", addr, i), token)
	})
}
