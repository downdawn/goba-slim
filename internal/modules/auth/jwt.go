package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	SessionID uuid.UUID `json:"sid"`
	Version   int64     `json:"ver"`
	jwt.RegisteredClaims
}

type Tokens struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string
	audience   string
	keyID      string
	ttl        time.Duration
}

func NewTokens(cfg config.AuthConfig) (*Tokens, error) {
	block, _ := pem.Decode([]byte(cfg.PrivateKey.Reveal()))
	if block == nil {
		return nil, fmt.Errorf("auth.private_key 必须是 Ed25519 PKCS#8 PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 Ed25519 私钥失败")
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("auth.private_key 不是 Ed25519 私钥")
	}
	return &Tokens{privateKey: privateKey, publicKey: privateKey.Public().(ed25519.PublicKey), issuer: cfg.Issuer, audience: cfg.Audience, keyID: cfg.KeyID, ttl: cfg.AccessTokenTTL}, nil
}

func (t *Tokens) Sign(userID, sessionID uuid.UUID, version int64, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(t.ttl)
	claims := Claims{SessionID: sessionID, Version: version, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: t.issuer, Audience: jwt.ClaimStrings{t.audience}, Subject: userID.String(),
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt), ID: uuid.NewString(),
	}}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = t.keyID
	signed, err := token.SignedString(t.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("签发 Access Token: %w", err)
	}
	return signed, expiresAt, nil
}

func (t *Tokens) Verify(encoded string, now time.Time) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(encoded, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() || token.Header["kid"] != t.keyID {
			return nil, ErrInvalidToken
		}
		return t.publicKey, nil
	}, jwt.WithIssuer(t.issuer), jwt.WithAudience(t.audience), jwt.WithTimeFunc(func() time.Time { return now }), jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	if err != nil || !parsed.Valid {
		return Claims{}, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.Subject == "" || claims.SessionID == uuid.Nil || claims.ID == "" {
		return Claims{}, ErrInvalidToken
	}
	return *claims, nil
}
