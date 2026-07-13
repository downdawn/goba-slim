package systemconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAndNormalizeAcceptsSupportedTypes(t *testing.T) {
	tests := []struct {
		name      string
		valueType ValueType
		value     string
		expected  string
	}{
		{name: "string", valueType: TypeString, value: `"hello"`, expected: `"hello"`},
		{name: "integer", valueType: TypeInteger, value: `9223372036854775807`, expected: `9223372036854775807`},
		{name: "boolean", valueType: TypeBoolean, value: `true`, expected: `true`},
		{name: "duration", valueType: TypeDuration, value: `"15m"`, expected: `"15m"`},
		{name: "string list", valueType: TypeStringList, value: `["a","b"]`, expected: `["a","b"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := ValidateAndNormalize(Input{Key: "feature.value", Value: json.RawMessage(test.value), ValueType: test.valueType})
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(input.Value))
		})
	}
}

func TestValidateAndNormalizeRejectsTypeMismatchAndSensitiveKeys(t *testing.T) {
	_, err := ValidateAndNormalize(Input{Key: "feature.limit", Value: json.RawMessage(`"not-number"`), ValueType: TypeInteger})
	require.ErrorIs(t, err, ErrInvalidType)
	_, err = ValidateAndNormalize(Input{Key: "feature.limit", Value: json.RawMessage(`1 trailing`), ValueType: TypeInteger})
	require.ErrorIs(t, err, ErrInvalidType)

	for _, key := range []string{"database.host", "auth.issuer", "feature.api_token", "service.private_key", "cors.allow_origins"} {
		_, err := ValidateAndNormalize(Input{Key: key, Value: json.RawMessage(`"value"`), ValueType: TypeString})
		require.ErrorIs(t, err, ErrSensitiveKey, key)
	}
}

func FuzzValidateAndNormalize(f *testing.F) {
	f.Add("feature.value", "string", `"hello"`)
	f.Add("auth.private_key", "string", `"secret"`)
	f.Fuzz(func(t *testing.T, key, valueType, value string) {
		_, _ = ValidateAndNormalize(Input{Key: key, Value: json.RawMessage(value), ValueType: ValueType(valueType)})
	})
}
