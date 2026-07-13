// Package systemconfig 提供非秘密、可在线调整的类型化业务配置。
package systemconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type ValueType string

const (
	TypeString     ValueType = "string"
	TypeInteger    ValueType = "integer"
	TypeBoolean    ValueType = "boolean"
	TypeDuration   ValueType = "duration"
	TypeStringList ValueType = "string_list"
)

type Config struct {
	Key         string
	Value       json.RawMessage
	ValueType   ValueType
	IsPublic    bool
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Input struct {
	Key         string
	Value       json.RawMessage
	ValueType   ValueType
	IsPublic    bool
	Description string
}

type ConfigChanged struct {
	Key       string
	Deleted   bool
	ChangedAt time.Time
}

var configKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,127}$`)

func ValidateAndNormalize(input Input) (Input, error) {
	input.Key = strings.TrimSpace(input.Key)
	input.Description = strings.TrimSpace(input.Description)
	if !configKeyPattern.MatchString(input.Key) || len(input.Description) > 255 || len(input.Value) == 0 || len(input.Value) > 64<<10 {
		return Input{}, ErrInvalidInput
	}
	if sensitiveKey(input.Key) {
		return Input{}, ErrSensitiveKey
	}
	value, err := normalizeValue(input.ValueType, input.Value)
	if err != nil {
		return Input{}, err
	}
	input.Value = value
	return input, nil
}

func normalizeValue(valueType ValueType, raw json.RawMessage) (json.RawMessage, error) {
	var value any
	switch valueType {
	case TypeString:
		value = new(string)
	case TypeInteger:
		var number int64
		value = &number
	case TypeBoolean:
		value = new(bool)
	case TypeDuration:
		var duration string
		if err := decodeStrict(raw, &duration); err != nil {
			return nil, ErrInvalidType
		}
		if _, err := time.ParseDuration(duration); err != nil {
			return nil, ErrInvalidType
		}
		encoded, _ := json.Marshal(duration)
		return encoded, nil
	case TypeStringList:
		var values []string
		if err := decodeStrict(raw, &values); err != nil || len(values) > 1000 {
			return nil, ErrInvalidType
		}
		for _, item := range values {
			if len(item) > 1024 {
				return nil, ErrInvalidType
			}
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("编码 string_list: %w", err)
		}
		return encoded, nil
	default:
		return nil, ErrInvalidType
	}
	if err := decodeStrict(raw, value); err != nil {
		return nil, ErrInvalidType
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("编码配置值: %w", err)
	}
	return encoded, nil
}

func decodeStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	err := decoder.Decode(&extra)
	if err == nil {
		return fmt.Errorf("配置值包含多个 JSON 值")
	}
	if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range []string{
		"app.", "server.", "database.", "redis.", "auth.", "cors.", "log.", "modules.", "file.", "systemconfig.",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	segments := strings.FieldsFunc(lower, func(char rune) bool { return char == '.' || char == '-' || char == '_' })
	for _, segment := range segments {
		switch segment {
		case "secret", "password", "passwd", "token", "cookie", "credential", "privatekey", "private", "key", "authorization", "dsn":
			return true
		}
	}
	return false
}
