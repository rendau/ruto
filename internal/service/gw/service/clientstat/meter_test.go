package clientstat

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
)

func trustedSnapshot(t *testing.T, trusted ...rootModel.RootTrustedClient) *rootModel.Root {
	t.Helper()
	snapshot := rootModel.NewEmpty()
	snapshot.TrustedClients = trusted
	require.NoError(t, snapshot.Normalize())
	return snapshot
}

func TestMeter_Start(t *testing.T) {
	c := NewCollector(100)
	m := New(c, clientip.New([]string{"127.0.0.0/8"}, nil)).Meter(trustedSnapshot(t,
		rootModel.RootTrustedClient{Name: "lan", Ip: "10.0.0.0/8"},
		rootModel.RootTrustedClient{Ip: "192.0.2.1"},
	))
	xff := func(ip string) http.Header { return http.Header{"X-Forwarded-For": {ip}} }

	direct := m.Start("203.0.113.7:1000", nil)
	assert.Equal(t, "203.0.113.7", direct.Addr().String())
	assert.Equal(t, "203.0.113.7", direct.Peer().String())

	proxied := m.Start("127.0.0.1:1000", xff("198.51.100.1"))
	assert.Equal(t, "198.51.100.1", proxied.Addr().String())
	assert.Equal(t, "127.0.0.1", proxied.Peer().String())
	proxied.Limited("ip 10/1m")
	proxied.Blocked()

	m.Start("127.0.0.1:1000", xff("10.1.2.3"))
	m.Start("127.0.0.1:1000", xff("10.200.0.1"))
	m.Start("127.0.0.1:1000", xff("192.0.2.1"))

	unknown := m.Start("127.0.0.1:1000", xff("not-an-ip"))
	assert.False(t, unknown.Addr().IsValid())
	assert.Equal(t, "127.0.0.1", unknown.Peer().String(), "rate limits fall back to the peer")
	m.Start("127.0.0.1:1000", nil)

	got := byIP(c.Drain())
	assert.Equal(t, &Item{Ip: "203.0.113.7", Requests: 1}, got["|203.0.113.7"])
	assert.Equal(t, &Item{Ip: "198.51.100.1", Requests: 1, Blocked: 1, Limited: 1, LastRule: "ip 10/1m"}, got["|198.51.100.1"])
	assert.Equal(t, &Item{Ip: "10.0.0.0/8", Trusted: "lan", Requests: 2}, got["lan|10.0.0.0/8"])
	assert.Equal(t, &Item{Ip: "192.0.2.1/32", Trusted: "192.0.2.1/32", Requests: 1}, got["192.0.2.1/32|192.0.2.1/32"],
		"an entry without a name is named by its network")
	assert.Equal(t, &Item{Requests: 2}, got["|"], "the client cannot be told")
	assert.Len(t, got, 5)
}

func TestMeter_NoTrustedClients(t *testing.T) {
	c := NewCollector(100)
	m := New(c, clientip.New(nil, nil)).Meter(rootModel.NewEmpty())
	m.Start("10.1.2.3:1000", nil)
	assert.Equal(t, []*Item{{Ip: "10.1.2.3", Requests: 1}}, c.Drain())
}

func TestMeter_Nil(t *testing.T) {
	var s *Service
	m := s.Meter(rootModel.NewEmpty())
	assert.Nil(t, m)

	client := m.Start("203.0.113.7:1000", nil)
	assert.Nil(t, client)
	assert.False(t, client.Addr().IsValid())
	client.Blocked()
	client.Limited("r")

	ctx := WithClient(context.Background(), client)
	assert.Nil(t, FromContext(ctx))
}

func TestContext(t *testing.T) {
	m := New(NewCollector(10), clientip.New(nil, nil)).Meter(nil)
	client := m.Start("203.0.113.7:1000", nil)
	assert.Same(t, client, FromContext(WithClient(context.Background(), client)))
	assert.Nil(t, FromContext(context.Background()))
}

func BenchmarkMeterStart(b *testing.B) {
	snapshot := rootModel.NewEmpty()
	snapshot.TrustedClients = []rootModel.RootTrustedClient{{Name: "lan", Ip: "10.0.0.0/8"}}
	require.NoError(b, snapshot.Normalize())
	m := New(NewCollector(MaxClients), clientip.New([]string{"127.0.0.0/8"}, nil)).Meter(snapshot)
	header := http.Header{"X-Forwarded-For": {"198.51.100.1"}}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Start("127.0.0.1:1000", header)
		}
	})
}
