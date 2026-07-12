package logging

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"unicode"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/shared/requestmeta"
)

const (
	redactedValue = "[REDACTED]"
	cycleValue    = "[CYCLE]"
	maxDepth      = 32
	maxNodes      = 1024
)

var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"authorization": {},
	"cookie":        {},
	"private_key":   {},
	"access_token":  {},
	"refresh_token": {},
	"id_token":      {},
	"api_key":       {},
	"client_secret": {},
	"secret":        {},
	"set_cookie":    {},
}

func New(cfg config.LogConfig, output io.Writer) (*slog.Logger, *slog.LevelVar) {
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.Set(slog.LevelInfo)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(output, options)
	} else {
		handler = slog.NewJSONHandler(output, options)
	}

	return slog.New(RedactAttrs(handler)), level
}

func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if requestID, ok := requestmeta.RequestID(ctx); ok && requestID != "" {
		return logger.With("request_id", requestID)
	}
	return logger
}

func RedactAttrs(handler slog.Handler) slog.Handler {
	return &redactingHandler{handler: handler}
}

type redactingHandler struct {
	handler slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	state := newRedactionState()
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(state.redactAttr(attr, 0))
		return true
	})
	return h.handler.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	state := newRedactionState()
	for i, attr := range attrs {
		redacted[i] = state.redactAttr(attr, 0)
	}
	return &redactingHandler{handler: h.handler.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{handler: h.handler.WithGroup(name)}
}

type redactionState struct {
	visited map[uintptr]struct{}
	nodes   int
}

func newRedactionState() *redactionState {
	return &redactionState{visited: make(map[uintptr]struct{})}
}

func (s *redactionState) redactAttr(attr slog.Attr, depth int) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if isSensitiveKey(attr.Key) {
		attr.Value = slog.StringValue(redactedValue)
		return attr
	}
	if depth >= maxDepth || s.nodes >= maxNodes {
		attr.Value = slog.StringValue(redactedValue)
		return attr
	}
	s.nodes++
	if attr.Value.Kind() == slog.KindGroup {
		source := attr.Value.Group()
		attrs := make([]slog.Attr, len(source))
		for i, child := range source {
			attrs[i] = s.redactAttr(child, depth+1)
		}
		attr.Value = slog.GroupValue(attrs...)
		return attr
	}
	if attr.Value.Kind() == slog.KindAny {
		attr.Value = slog.AnyValue(s.redactReflectValue(reflect.ValueOf(attr.Value.Any()), depth+1))
	}
	return attr
}

func (s *redactionState) redactReflectValue(value reflect.Value, depth int) any {
	if !value.IsValid() {
		return nil
	}
	if depth >= maxDepth || s.nodes >= maxNodes {
		return redactedValue
	}
	s.nodes++
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return value.Interface()
		}
		return s.redactReflectValue(value.Elem(), depth+1)
	}
	if value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String {
		return value.Interface()
	}
	if value.IsNil() {
		return value.Interface()
	}

	pointer := uintptr(value.UnsafePointer())
	if _, ok := s.visited[pointer]; ok {
		return cycleValue
	}
	s.visited[pointer] = struct{}{}
	defer delete(s.visited, pointer)

	redacted := make(map[string]any, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		key := iterator.Key().String()
		if isSensitiveKey(key) {
			redacted[key] = redactedValue
		} else {
			redacted[key] = s.redactReflectValue(iterator.Value(), depth+1)
		}
	}
	return redacted
}

func isSensitiveKey(key string) bool {
	_, ok := sensitiveKeys[normalizeKey(key)]
	return ok
}

func normalizeKey(key string) string {
	var normalized strings.Builder
	lastSeparator := false
	for _, r := range strings.ToLower(strings.TrimSpace(key)) {
		if r == '-' || r == '.' || unicode.IsSpace(r) {
			if !lastSeparator {
				normalized.WriteByte('_')
				lastSeparator = true
			}
			continue
		}
		normalized.WriteRune(r)
		lastSeparator = false
	}
	return strings.Trim(normalized.String(), "_")
}
