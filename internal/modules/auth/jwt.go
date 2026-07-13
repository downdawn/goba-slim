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
	publicKeys map[string]ed25519.PublicKey
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
	publicKeys := map[string]ed25519.PublicKey{cfg.KeyID: privateKey.Public().(ed25519.PublicKey)}
	for keyID, encoded := range cfg.VerificationKeys {
		if keyID == cfg.KeyID {
			return nil, fmt.Errorf("auth.verification_key_files 不能包含当前 key_id")
		}
		publicKey, err := parsePublicKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("解析验证公钥 %s 失败: %w", keyID, err)
		}
		publicKeys[keyID] = publicKey
	}
	return &Tokens{privateKey: privateKey, publicKeys: publicKeys, issuer: cfg.Issuer, audience: cfg.Audience, keyID: cfg.KeyID, ttl: cfg.AccessTokenTTL}, nil
}

func parsePublicKey(encoded string) (ed25519.PublicKey, error) {
	block, rest := pem.Decode([]byte(encoded))
	if block == nil || len(rest) != 0 {
		return nil, fmt.Errorf("必须是 Ed25519 PKIX PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("必须是 Ed25519 PKIX PEM")
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("不是 Ed25519 公钥")
	}
	return publicKey, nil
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
		if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, ErrInvalidToken
		}
		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" {
			return nil, ErrInvalidToken
		}
		publicKey, ok := t.publicKeys[keyID]
		if !ok {
			return nil, ErrInvalidToken
		}
		return publicKey, nil
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
