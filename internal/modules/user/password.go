package user

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argonVersion      = 19
	maxPasswordBytes  = 512
	maxArgonMemoryKiB = 256 * 1024
	maxArgonTime      = 10
	maxArgonThreads   = 16
)

type Argon2Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltLen   uint32
	KeyLen    uint32
}

func DefaultArgon2Params() Argon2Params {
	return Argon2Params{MemoryKiB: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}
}

type PasswordManager interface {
	Validate(string) error
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
	NeedsRehash(string) bool
}

type Passwords struct{ params Argon2Params }

func NewPasswords(params Argon2Params) (*Passwords, error) {
	if err := validateArgon2Params(params); err != nil {
		return nil, err
	}
	return &Passwords{params: params}, nil
}

func (p *Passwords) Validate(password string) error {
	if !utf8.ValidString(password) || len(password) > maxPasswordBytes {
		return ErrInvalidPassword
	}
	runes := []rune(password)
	if len(runes) < 8 || len(runes) > 128 {
		return ErrInvalidPassword
	}
	var hasLetter, hasDigit bool
	for _, item := range runes {
		hasLetter = hasLetter || unicode.IsLetter(item)
		hasDigit = hasDigit || unicode.IsDigit(item)
	}
	if !hasLetter || !hasDigit || weakPasswords[strings.ToLower(password)] {
		return ErrInvalidPassword
	}
	return nil
}

func (p *Passwords) Hash(password string) (string, error) {
	if err := p.Validate(password); err != nil {
		return "", err
	}
	salt := make([]byte, p.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成密码 Salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, p.params.Time, p.params.MemoryKiB, p.params.Threads, p.params.KeyLen)
	return encodePHC(p.params, salt, hash), nil
}

func (p *Passwords) Verify(password, encoded string) (bool, error) {
	params, salt, expected, err := decodePHC(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.Time, params.MemoryKiB, params.Threads, params.KeyLen)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (p *Passwords) NeedsRehash(encoded string) bool {
	params, _, _, err := decodePHC(encoded)
	return err != nil || params != p.params
}

func encodePHC(params Argon2Params, salt, hash []byte) string {
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argonVersion, params.MemoryKiB, params.Time, params.Threads, encoding.EncodeToString(salt), encoding.EncodeToString(hash))
}

func decodePHC(encoded string) (Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || len(encoded) > 1024 {
		return Argon2Params{}, nil, nil, fmt.Errorf("无效的 Argon2id 摘要")
	}
	values := strings.Split(parts[3], ",")
	if len(values) != 3 {
		return Argon2Params{}, nil, nil, fmt.Errorf("无效的 Argon2id 参数")
	}
	memory, err := parsePHCValue(values[0], "m=")
	if err != nil {
		return Argon2Params{}, nil, nil, err
	}
	timeCost, err := parsePHCValue(values[1], "t=")
	if err != nil {
		return Argon2Params{}, nil, nil, err
	}
	threads, err := parsePHCValue(values[2], "p=")
	if err != nil || threads > 255 {
		return Argon2Params{}, nil, nil, fmt.Errorf("无效的 Argon2id 并行度")
	}
	encoding := base64.RawStdEncoding
	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("解析 Argon2id Salt: %w", err)
	}
	hash, err := encoding.DecodeString(parts[5])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("解析 Argon2id 摘要: %w", err)
	}
	if memory > maxArgonMemoryKiB || timeCost > maxArgonTime || threads > maxArgonThreads || len(salt) > 64 || len(hash) > 64 {
		return Argon2Params{}, nil, nil, fmt.Errorf("Argon2id 参数超出安全范围")
	}
	// #nosec G115 -- 上述硬上限保证所有转换均在目标类型范围内。
	params := Argon2Params{MemoryKiB: uint32(memory), Time: uint32(timeCost), Threads: uint8(threads), SaltLen: uint32(len(salt)), KeyLen: uint32(len(hash))}
	if err := validateArgon2Params(params); err != nil {
		return Argon2Params{}, nil, nil, err
	}
	return params, salt, hash, nil
}

func parsePHCValue(value, prefix string) (uint64, error) {
	raw, ok := strings.CutPrefix(value, prefix)
	if !ok || raw == "" {
		return 0, fmt.Errorf("无效的 Argon2id 参数")
	}
	parsed, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("解析 Argon2id 参数: %w", err)
	}
	return parsed, nil
}

func validateArgon2Params(params Argon2Params) error {
	if params.MemoryKiB < 8*1024 || params.MemoryKiB > maxArgonMemoryKiB || params.Time < 1 || params.Time > maxArgonTime || params.Threads < 1 || params.Threads > maxArgonThreads || params.SaltLen < 8 || params.SaltLen > 64 || params.KeyLen < 16 || params.KeyLen > 64 {
		return fmt.Errorf("Argon2id 参数超出安全范围")
	}
	return nil
}

var weakPasswords = map[string]bool{
	"password1": true, "password123": true, "admin123": true, "qwerty123": true,
	"abc12345": true, "letmein123": true, "welcome123": true, "iloveyou1": true,
}
