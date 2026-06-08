package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

type JWKSGenerateCmd struct {
	Key  string `help:"Output private key PEM file." default:"../certs/jwt-signing-key.pem"`
	JWKS string `help:"Output JWKS file." default:"../certs/jwks.json"`
}

type JWKSCmd struct {
	Generate JWKSGenerateCmd `cmd:"generate" help:"Generate signing key and JWKS file."`
}

type JWTSignCmd struct {
	Key      string        `help:"Private key PEM file." default:"../certs/jwt-signing-key.pem"`
	Issuer   string        `help:"Token issuer." default:"https://example.com"`
	Audience string        `help:"Token audience." default:"echoserver"`
	Subject  string        `help:"Token subject." default:"user"`
	Expiry   time.Duration `help:"Token lifetime." default:"1h"`
}

type JWTCmd struct {
	Sign JWTSignCmd `cmd:"sign" help:"Generate a signed JWT token."`
}

func runJWKSGenerate(cmd *JWKSGenerateCmd) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating key: %v\n", err)
		os.Exit(1)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling key: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(cmd.Key, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing key: %v\n", err)
		os.Exit(1)
	}

	jwks := map[string]any{
		"keys": []map[string]any{{
			"kty": "EC",
			"crv": "P-256",
			"x":   jwksB64url(key.PublicKey.X),
			"y":   jwksB64url(key.PublicKey.Y),
			"use": "sig",
			"alg": "ES256",
			"kid": "1",
		}},
	}
	out, _ := json.MarshalIndent(jwks, "", "  ")
	if err := os.WriteFile(cmd.JWKS, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing jwks: %v\n", err)
		os.Exit(1)
	}
}

func runJWTSign(cmd *JWTSignCmd) {
	data, err := os.ReadFile(cmd.Key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading key: %v\n", err)
		os.Exit(1)
	}
	block, _ := pem.Decode(data)
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing key: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT", "kid": "1"})
	payload, _ := json.Marshal(map[string]any{
		"iss": cmd.Issuer,
		"aud": cmd.Audience,
		"sub": cmd.Subject,
		"iat": now.Unix(),
		"exp": now.Add(cmd.Expiry).Unix(),
	})

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error signing: %v\n", err)
		os.Exit(1)
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	fmt.Println(signingInput + "." + base64.RawURLEncoding.EncodeToString(sig))
}

func jwksB64url(n *big.Int) string {
	b := make([]byte, 32)
	n.FillBytes(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
