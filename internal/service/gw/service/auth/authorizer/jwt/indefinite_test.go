package jwt

import (
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestMarkIndefinite(t *testing.T) {
	const kid = "kid-indefinite-test"

	// токен без exp: первое появление фиксируется, повтор — нет
	userId, firstSeen := markIndefinite(kid, jwtv5.MapClaims{"sub": "u-no-exp"})
	require.Equal(t, "u-no-exp", userId)
	require.True(t, firstSeen)

	_, firstSeen = markIndefinite(kid, jwtv5.MapClaims{"sub": "u-no-exp"})
	require.False(t, firstSeen)

	// токен с exp игнорируется
	userId, firstSeen = markIndefinite(kid, jwtv5.MapClaims{"sub": "u-with-exp", "exp": float64(1)})
	require.Equal(t, "", userId)
	require.False(t, firstSeen)

	// пустой sub без exp фиксируется как "<unknown>"
	userId, firstSeen = markIndefinite(kid, jwtv5.MapClaims{})
	require.Equal(t, "<unknown>", userId)
	require.True(t, firstSeen)
}
