package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/tsaarni/echoclient/worker"
)

func runTransit(addr, token string, count, rps, concurrency, size int) {
	slog.Info("Starting transit", "count", humanize.Comma(int64(count)), "size", humanize.IBytes(uint64(size)))

	plaintext := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("A", size)))
	encPayload := []byte(fmt.Sprintf(`{"plaintext":"%s"}`, plaintext))

	run(count, rps, concurrency, func(ctx context.Context, _ *worker.WorkerPool) error {
		// Encrypt
		resp, err := doReqCtx(ctx, "POST", addr+"/v1/transit/encrypt/default", token, encPayload)
		if err != nil {
			return err
		}
		var encResp struct {
			Data struct {
				Ciphertext string `json:"ciphertext"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&encResp)
		resp.Body.Close()

		// Decrypt
		decPayload := []byte(fmt.Sprintf(`{"ciphertext":"%s"}`, encResp.Data.Ciphertext))
		return post(ctx, addr+"/v1/transit/decrypt/default", token, decPayload)
	})
}
