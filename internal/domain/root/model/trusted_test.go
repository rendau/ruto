package model

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClientPrefix(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":             "203.0.113.7/32",
		" 203.0.113.9/24 ":        "203.0.113.0/24",
		"2001:db8:1:2:3:4:5:6/64": "2001:db8:1:2::/64",
		"::ffff:203.0.113.7":      "203.0.113.7/32",
		"::ffff:203.0.113.0/120":  "203.0.113.0/24",
		"fe80::1%eth0":            "fe80::1/128",
		"2001:db8::1":             "2001:db8::1/128",
	}
	for raw, want := range cases {
		got, err := ParseClientPrefix(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got.String(), raw)
	}
	for _, raw := range []string{"", "localhost", "1.2.3.4/33", "1.2.3"} {
		_, err := ParseClientPrefix(raw)
		assert.Error(t, err, raw)
	}
}

func TestTrustedClients(t *testing.T) {
	root := NewEmpty()
	root.TrustedClients = []RootTrustedClient{{Name: " lan ", Ip: "192.168.0.0/16"}, {Name: "1c", Ip: "10.1.2.3"}}

	require.NoError(t, root.Normalize())

	assert.Equal(t, "lan", root.TrustedClients[0].Name)
	assert.Equal(t, "10.1.2.3/32", root.TrustedClients[1].Ip)
	got, ok := root.TrustedPrefix(netip.MustParseAddr("192.168.62.3"))
	require.True(t, ok)
	assert.Equal(t, "lan", got.Name)
	_, ok = root.TrustedPrefix(netip.MustParseAddr("::ffff:10.1.2.3"))
	assert.True(t, ok)
	_, ok = root.TrustedPrefix(netip.MustParseAddr("203.0.113.7"))
	assert.False(t, ok)

	for _, ip := range []string{"nope", "0.0.0.0/0", "10.0.0.0/7", "::/0", "2001:db8::/31"} {
		root.TrustedClients = []RootTrustedClient{{Ip: ip}}
		assert.Error(t, root.Normalize(), ip)
	}
	root.TrustedClients = []RootTrustedClient{{Ip: "10.0.0.0/8"}, {Ip: "2001:db8::/32"}}
	assert.NoError(t, root.Normalize())
}
