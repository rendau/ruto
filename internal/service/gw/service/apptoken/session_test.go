package apptoken

import (
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestSameSession(t *testing.T) {
	tests := []struct {
		name          string
		shared, token jwtv5.MapClaims
		want          bool
	}{
		{"same person, same session", jwtv5.MapClaims{"sub": "alice", "sid": "s1"}, jwtv5.MapClaims{"sub": "alice", "sid": "s1"}, true},
		{"same person, earlier login", jwtv5.MapClaims{"sub": "alice", "sid": "s2"}, jwtv5.MapClaims{"sub": "alice", "sid": "s1"}, false},
		{"another person", jwtv5.MapClaims{"sub": "bob", "sid": "s3"}, jwtv5.MapClaims{"sub": "alice", "sid": "s1"}, false},
		{"provider without sid, same person", jwtv5.MapClaims{"sub": "alice"}, jwtv5.MapClaims{"sub": "alice"}, true},
		{"provider without sid, another person", jwtv5.MapClaims{"sub": "bob"}, jwtv5.MapClaims{"sub": "alice"}, false},
		{"the token names no session", jwtv5.MapClaims{"sub": "alice", "sid": "s1"}, jwtv5.MapClaims{"sub": "alice"}, true},
		{"no person in the shared token", jwtv5.MapClaims{}, jwtv5.MapClaims{}, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, SameSession(tt.shared, tt.token), tt.name)
	}
}
