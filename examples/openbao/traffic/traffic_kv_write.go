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

func runKVWrite(addr, token, engine string, count, rps, concurrency, size int) {
	mount := "kv" + engine
	if engine == "2" {
		mount = "kv2/metadata"
	}
	offset := getKeyCount(addr, token, mount)
	total := int64(count) * int64(size)
	slog.Info("Starting kv-write", "engine", engine, "count", humanize.Comma(int64(count)), "size", humanize.IBytes(uint64(size)), "total", humanize.IBytes(uint64(total)), "existing", humanize.Comma(int64(offset)))

	var payload []byte
	if engine == "2" {
		payload = generateKV2Payload(size)
	} else {
		payload = generateKV1Payload(size)
	}
	var counter atomic.Int64
	counter.Store(int64(offset))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		i := counter.Add(1)
		var url string
		if engine == "2" {
			url = fmt.Sprintf("%s/v1/kv2/data/key-%d", addr, i)
		} else {
			url = fmt.Sprintf("%s/v1/kv1/key-%d", addr, i)
		}
		return post(ctx, url, token, payload)
	})
}

func generateKV2Payload(size int) []byte {
	r := generator.NewReader(generator.WithASCII(), generator.WithTotalSize(uint64(size)))
	data, _ := io.ReadAll(r)
	return []byte(fmt.Sprintf(`{"data":{"value":"%s"}}`, base64.StdEncoding.EncodeToString(data)))
}

func generateKV1Payload(size int) []byte {
	r := generator.NewReader(generator.WithASCII(), generator.WithTotalSize(uint64(size)))
	data, _ := io.ReadAll(r)
	return []byte(fmt.Sprintf(`{"value":"%s"}`, base64.StdEncoding.EncodeToString(data)))
}
