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
	"github.com/stretchr/testify/require"
)

func TestTokensSignAndVerifyEdDSAClaims(t *testing.T) {
	tokens := testTokens(t)
	now := time.Date(2026, time.July, 12, 15, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("019befd7-98d0-7000-8000-000000000010")
	sessionID := uuid.MustParse("019befd7-98d0-7000-8000-000000000011")

	encoded, expiresAt, err := tokens.Sign(userID, sessionID, 3, now)
	require.NoError(t, err)
	claims, err := tokens.Verify(encoded, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, userID.String(), claims.Subject)
	require.Equal(t, sessionID, claims.SessionID)
	require.Equal(t, int64(3), claims.Version)
	require.Equal(t, now.Add(15*time.Minute), expiresAt)
	_, err = tokens.Verify(encoded, expiresAt.Add(time.Second))
	require.ErrorIs(t, err, ErrInvalidToken)
}

func testTokens(t *testing.T) *Tokens {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	cfg := config.Default().Auth
	cfg.PrivateKey = config.NewSecret(string(block))
	tokens, err := NewTokens(cfg)
	require.NoError(t, err)
	return tokens
}
