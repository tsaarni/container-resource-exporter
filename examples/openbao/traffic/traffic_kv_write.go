package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/generator"
	"github.com/tsaarni/echoclient/worker"
)

func runKVWrite(addr, token string, count, rps, concurrency, size int) {
	// Uses KV v1 at secret/
	offset := getKeyCount(addr, token, "secret")
	total := int64(count) * int64(size)
	slog.Info("Starting kv-write", "count", humanize.Comma(int64(count)), "size", humanize.IBytes(uint64(size)), "total", humanize.IBytes(uint64(total)), "existing", humanize.Comma(int64(offset)))

	payload := generateKV1Payload(size)
	var counter atomic.Int64
	counter.Store(int64(offset))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		i := counter.Add(1)
		return post(ctx, fmt.Sprintf("%s/v1/secret/key-%d", addr, i), token, payload)
	})
}

func generateKV1Payload(size int) []byte {
	r := generator.NewReader(generator.WithASCII(), generator.WithTotalSize(uint64(size)))
	data, _ := io.ReadAll(r)
	return []byte(fmt.Sprintf(`{"value":"%s"}`, base64.StdEncoding.EncodeToString(data)))
}
