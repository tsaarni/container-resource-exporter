package main

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/tsaarni/echoclient/worker"
)

func runCertAuth(cmd CertAuthCmd) {
	cert, err := tls.LoadX509KeyPair(cmd.Cert, cmd.Key)
	if err != nil {
		log.Fatalf("load client cert: %v", err)
	}
	httpClient := newHTTPClient(&tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	})
	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		return doGet(ctx, &httpClient, "https://echoserver-cert-auth.127.0.0.1.nip.io/")
	})
}
