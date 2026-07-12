package user

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordsHashVerifyAndDetectRehash(t *testing.T) {
	passwords, err := NewPasswords(testArgon2Params())
	require.NoError(t, err)

	encoded, err := passwords.Hash("CorrectHorse9")
	require.NoError(t, err)
	matched, err := passwords.Verify("CorrectHorse9", encoded)
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = passwords.Verify("WrongHorse9", encoded)
	require.NoError(t, err)
	require.False(t, matched)
	require.False(t, passwords.NeedsRehash(encoded))
}

func TestPasswordsRejectWeakAndHostileInputs(t *testing.T) {
	passwords, err := NewPasswords(testArgon2Params())
	require.NoError(t, err)

	for _, value := range []string{"short1", "password123", strings.Repeat("a", 129) + "1"} {
		require.ErrorIs(t, passwords.Validate(value), ErrInvalidPassword)
	}
	_, err = passwords.Verify("SomePassword9", "$argon2id$v=19$m=999999999,t=3,p=2$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA")
	require.Error(t, err)
}

func testArgon2Params() Argon2Params {
	return Argon2Params{MemoryKiB: 8 * 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}
}
