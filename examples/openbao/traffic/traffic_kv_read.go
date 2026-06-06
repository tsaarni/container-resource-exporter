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

func runKVRead(addr, token, engine string, count, rps, concurrency, pool int) {
	mount := "kv" + engine
	if engine == "2" {
		mount = "kv2/metadata"
	}
	existing := getKeyCount(addr, token, mount)
	if existing == 0 {
		log.Fatal("No secrets to read. Run kv-write first.")
	}
	if pool > existing {
		pool = existing
	}
	slog.Info("Starting kv-read", "engine", engine, "count", humanize.Comma(int64(count)), "pool", humanize.Comma(int64(pool)))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		i := rand.IntN(pool) + 1
		var url string
		if engine == "2" {
			url = fmt.Sprintf("%s/v1/kv2/data/key-%d", addr, i)
		} else {
			url = fmt.Sprintf("%s/v1/kv1/key-%d", addr, i)
		}
		return get(ctx, url, token)
	})
}
