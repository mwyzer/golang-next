package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang-nextjs/internal/auth"
)

func TestGenerateToken(t *testing.T) {
	token, hash, err := auth.GenerateToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Len(t, token, 64) // 32 random bytes, hex-encoded
	assert.Equal(t, auth.HashToken(token), hash)

	token2, hash2, err := auth.GenerateToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, token2, "tokens must be unique")
	assert.NotEqual(t, hash, hash2)
}

func TestHashToken(t *testing.T) {
	assert.Equal(t, auth.HashToken("dev-token"), auth.HashToken("dev-token"),
		"hashing must be deterministic")
	assert.NotEqual(t, auth.HashToken("dev-token"), auth.HashToken("other-token"))
	assert.NotContains(t, auth.HashToken("dev-token"), "dev-token",
		"the hash must not leak the plaintext")
}
