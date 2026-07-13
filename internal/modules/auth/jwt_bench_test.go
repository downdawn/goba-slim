package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/google/uuid"
)

func BenchmarkTokensVerify(b *testing.B) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		b.Fatal(err)
	}
	cfg := config.Default().Auth
	cfg.PrivateKey = config.NewSecret(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})))
	tokens, err := NewTokens(cfg)
	if err != nil {
		b.Fatal(err)
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	token, _, err := tokens.Sign(uuid.New(), uuid.New(), 1, now)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := tokens.Verify(token, now.Add(time.Minute)); err != nil {
			b.Fatal(err)
		}
	}
}
