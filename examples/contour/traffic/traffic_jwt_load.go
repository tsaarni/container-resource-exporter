package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"log"
	"os"
	"time"

	"github.com/tsaarni/echoclient/worker"
)

func runJWTLoad(cmd JWTLoadCmd) {
	keyData, err := os.ReadFile(cmd.SigningKey)
	if err != nil {
		log.Fatalf("read signing key: %v", err)
	}
	block, _ := pem.Decode(keyData)
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("parse signing key: %v", err)
	}

	httpClient := newHTTPClient(&tls.Config{InsecureSkipVerify: true})

	run(cmd.workerFlags, func(ctx context.Context, _ *worker.WorkerPool) error {
		token := signJWTToken(key, cmd.Issuer, cmd.Audience, cmd.Subject)
		return doGetWithHeaders(ctx, &httpClient, "https://echoserver-jwt.127.0.0.1.nip.io/", map[string]string{
			"Authorization": "Bearer " + token,
		})
	})
}

func signJWTToken(key *ecdsa.PrivateKey, issuer, audience, subject string) string {
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT", "kid": "1"})
	payload, _ := json.Marshal(map[string]any{
		"iss": issuer,
		"aud": audience,
		"sub": subject,
		"iat": now.Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	})

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return ""
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}
