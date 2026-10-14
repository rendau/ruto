package secretref

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const appJSON = `{
	"name": "orders",
	"backend": {"url": "http://orders", "headers": {"X-Token": "header-secret"}, "query_params": {"sig": "q"}},
	"variables": {"DB_PASS": "пароль-123"},
	"auth": {"methods": [
		{"basic": {"users": [{"username": "bob", "password": "bob-pass"}]}},
		{"api_key": {"header": "X-Api-Key", "keys": [{"name": "mobile", "key": "api-secret"}]}},
		{"jwt": {"kid": "k1", "roles": ["admin"]}}
	]}
}`

func tree(t *testing.T, raw string) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	return result
}

func TestMask_HidesEverySecretAndKeepsTheRest(t *testing.T) {
	masked := Mask(KindApp, "a1", tree(t, appJSON))

	raw, err := json.Marshal(masked)
	require.NoError(t, err)
	for _, secret := range []string{"header-secret", `"q"`, "пароль-123", "bob-pass", "api-secret"} {
		assert.NotContains(t, string(raw), secret)
	}
	for _, plain := range []string{"http://orders", "bob", "mobile", "X-Api-Key", "k1"} {
		assert.Contains(t, string(raw), plain)
	}

	m := masked.(map[string]any)
	pass := m["variables"].(map[string]any)["DB_PASS"].(map[string]any)
	assert.Equal(t, "app:a1/variables/DB_PASS", pass["secret_ref"])
	assert.Equal(t, 10, pass["chars"], "length is counted in characters, not bytes")

	key := m["auth"].(map[string]any)["methods"].([]any)[1].(map[string]any)["api_key"].(map[string]any)["keys"].([]any)[0].(map[string]any)["key"].(map[string]any)
	assert.Equal(t, "app:a1/auth/methods/1/api_key/keys/0/key", key["secret_ref"])
}

func TestRef_RoundTripAndLookup(t *testing.T) {
	data := tree(t, appJSON)

	for _, s := range []string{
		"app:a1/variables/DB_PASS",
		"app:a1/backend/headers/X-Token",
		"app:a1/auth/methods/0/basic/users/0/password",
		"root:/variables/X",
		"endpoint:e1/backend/query_params/sig",
	} {
		ref, err := ParseRef(s)
		require.NoError(t, err, s)
		assert.Equal(t, s, ref.String())
	}

	ref, err := ParseRef("app:a1/auth/methods/1/api_key/keys/0/key")
	require.NoError(t, err)
	value, ok := Lookup(data, ref.Path)
	require.True(t, ok)
	assert.Equal(t, "api-secret", value)
}

func TestParseRef_RejectsNonSecretPaths(t *testing.T) {
	for _, s := range []string{
		"app:a1/name",
		"app:a1/backend/url",
		"app:a1/auth/methods/0/basic/users/0/username",
		"root:x/variables/A",
		"app:/variables/A",
		"secret:a/variables/A",
		"no-kind",
	} {
		_, err := ParseRef(s)
		assert.Error(t, err, s)
	}
}

func TestParseSource(t *testing.T) {
	src, err := ParseSource("plain")
	require.NoError(t, err)
	require.NotNil(t, src.Literal)
	assert.Equal(t, "plain", *src.Literal)

	src, err = ParseSource(map[string]any{"secret_ref": "app:a1/variables/X", "chars": 5.0})
	require.NoError(t, err)
	assert.Equal(t, "app:a1/variables/X", src.Ref.String())

	src, err = ParseSource(map[string]any{"generate": map[string]any{"format": "hex", "length": 10.0}, "name": "t"})
	require.NoError(t, err)
	assert.Equal(t, &Generate{Format: "hex", Length: 10, Name: "t"}, src.Generate)

	src, err = ParseSource(map[string]any{"reuse": "t"})
	require.NoError(t, err)
	assert.Equal(t, "t", src.Reuse)

	for _, bad := range []any{
		map[string]any{"value": "x"},
		map[string]any{"generate": map[string]any{"format": "emoji"}},
		map[string]any{"generate": map[string]any{"length": 100000.0}},
		map[string]any{"reuse": ""},
		12.0,
	} {
		_, err = ParseSource(bad)
		assert.Error(t, err, bad)
	}
}

func TestGenerate_Formats(t *testing.T) {
	for format, check := range map[string]func(string) bool{
		"alnum":     func(v string) bool { return strings.Trim(v, alnumChars) == "" },
		"digits":    func(v string) bool { return strings.Trim(v, digitChars) == "" },
		"hex":       func(v string) bool { return strings.Trim(v, "0123456789abcdef") == "" },
		"base64url": func(v string) bool { return !strings.ContainsAny(v, "+/=") },
	} {
		gen := &Generate{Format: format, Length: 33}
		value, err := gen.Value()
		require.NoError(t, err)
		assert.Len(t, value, 33, format)
		assert.True(t, check(value), format)
	}

	gen := &Generate{Format: "uuid"}
	value, err := gen.Value()
	require.NoError(t, err)
	assert.Len(t, value, gen.Chars())
	assert.Equal(t, byte('4'), value[14])
}

func TestRewrite_TouchesOnlySecretLeaves(t *testing.T) {
	var seen []string
	result, err := Rewrite(tree(t, appJSON), func(path []string, node any) (any, error) {
		seen = append(seen, FormatPath(path))
		return "x", nil
	})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"backend.headers.X-Token",
		"backend.query_params.sig",
		"variables.DB_PASS",
		"auth.methods[0].basic.users[0].password",
		"auth.methods[1].api_key.keys[0].key",
	}, seen)
	assert.Equal(t, "http://orders", result.(map[string]any)["backend"].(map[string]any)["url"])
}

func TestVersion_ChangesWithContent(t *testing.T) {
	a, err := Version(map[string]any{"a": 1})
	require.NoError(t, err)
	b, err := Version(map[string]any{"a": 2})
	require.NoError(t, err)
	c, err := Version(map[string]any{"a": 1})
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
	assert.Equal(t, a, c)
}
