package authflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

func TestSealedCookie(t *testing.T) {
	SetCookieKey(snapshotModel.CookieKeyFrom("k1"))
	t.Cleanup(func() { SetCookieKey(nil) })

	session := sessionCookie{RefreshToken: "rt", IdToken: "idt"}
	value, ok := session.encode("ruto-rt_corp")
	require.True(t, ok)
	assert.NotContains(t, value, "rt", "nothing readable in the cookie")

	assert.Equal(t, session, decodeSessionCookie("ruto-rt_corp", value))

	again, _ := session.encode("ruto-rt_corp")
	assert.NotEqual(t, value, again, "a fresh nonce every time")

	assert.Empty(t, decodeSessionCookie("ruto-rt_corp_shop", value), "sealed for another cookie")
	assert.Empty(t, decodeSessionCookie("ruto-rt_corp", value[:len(value)-2]+"AA"), "tampered")
	assert.Empty(t, decodeSessionCookie("ruto-rt_corp", "not-base64!"), "garbage")
	assert.Empty(t, decodeSessionCookie("ruto-rt_corp", ""), "empty")

	SetCookieKey(snapshotModel.CookieKeyFrom("k2"))
	assert.Empty(t, decodeSessionCookie("ruto-rt_corp", value), "another key: the session is gone")

	SetCookieKey([]byte("short"))
	_, ok = session.encode("ruto-rt_corp")
	assert.False(t, ok, "a key of the wrong size is no key")

	SetCookieKey(nil)
	_, ok = session.encode("ruto-rt_corp")
	assert.False(t, ok, "no key, no cookie")
}
