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

func runBulkDelete(addr, token string, rps, concurrency int) {
	// KV v1: no pagination support, re-list after each batch
	deleteMount(addr, token, "kv1", "/v1/kv1/", rps, concurrency)
	// KV v2: use after+limit pagination to avoid fetching all keys at once
	deleteMountPaginated(addr, token, "kv2/metadata", "/v1/kv2/metadata/", rps, concurrency)
}

const deletePageSize = 10000

func deleteMount(addr, token, listMount, deletePrefix string, rps, concurrency int) {
	for {
		keys := listKeys(addr, token, listMount)
		if len(keys) == 0 {
			return
		}
		slog.Debug("Deleting batch", "mount", listMount, "count", humanize.Comma(int64(len(keys))))
		var idx atomic.Int64
		run(len(keys), rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
			i := int(idx.Add(1)) - 1
			resp, err := doReqCtx(ctx, "DELETE", fmt.Sprintf("%s%s%s", addr, deletePrefix, keys[i]), token, nil)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		})
	}
}

func deleteMountPaginated(addr, token, listMount, deletePrefix string, rps, concurrency int) {
	for {
		keys := listKeysPage(addr, token, listMount, "", deletePageSize)
		if len(keys) == 0 {
			return
		}
		slog.Debug("Deleting batch", "mount", listMount, "count", humanize.Comma(int64(len(keys))))
		var idx atomic.Int64
		run(len(keys), rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
			i := int(idx.Add(1)) - 1
			resp, err := doReqCtx(ctx, "DELETE", fmt.Sprintf("%s%s%s", addr, deletePrefix, keys[i]), token, nil)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		})
	}
}
