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

func TestTokensVerifyPreviousPublicKeyDuringRotation(t *testing.T) {
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("019befd7-98d0-7000-8000-000000000020")
	sessionID := uuid.MustParse("019befd7-98d0-7000-8000-000000000021")
	oldPrivate, oldPublic := generateKeyPair(t)
	oldConfig := config.Default().Auth
	oldConfig.KeyID = "old"
	oldConfig.PrivateKey = config.NewSecret(oldPrivate)
	oldTokens, err := NewTokens(oldConfig)
	require.NoError(t, err)
	oldEncoded, _, err := oldTokens.Sign(userID, sessionID, 1, now)
	require.NoError(t, err)

	newPrivate, _ := generateKeyPair(t)
	newConfig := config.Default().Auth
	newConfig.KeyID = "new"
	newConfig.PrivateKey = config.NewSecret(newPrivate)
	newConfig.VerificationKeys = map[string]string{"old": oldPublic}
	newTokens, err := NewTokens(newConfig)
	require.NoError(t, err)

	claims, err := newTokens.Verify(oldEncoded, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, userID.String(), claims.Subject)
	newEncoded, _, err := newTokens.Sign(userID, sessionID, 1, now)
	require.NoError(t, err)
	_, err = oldTokens.Verify(newEncoded, now.Add(time.Minute))
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokensRejectInvalidVerificationKey(t *testing.T) {
	privateKey, _ := generateKeyPair(t)
	cfg := config.Default().Auth
	cfg.PrivateKey = config.NewSecret(privateKey)
	cfg.VerificationKeys = map[string]string{"old": privateKey}

	_, err := NewTokens(cfg)
	require.ErrorContains(t, err, "Ed25519 PKIX PEM")
}

func testTokens(t *testing.T) *Tokens {
	t.Helper()
	privateKey, _ := generateKeyPair(t)
	cfg := config.Default().Auth
	cfg.PrivateKey = config.NewSecret(privateKey)
	tokens, err := NewTokens(cfg)
	require.NoError(t, err)
	return tokens
}

func generateKeyPair(t *testing.T) (string, string) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	require.NoError(t, err)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM)
}
