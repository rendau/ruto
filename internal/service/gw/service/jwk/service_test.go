package jwk

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"encoding/json"

	"github.com/stretchr/testify/require"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

func newTestService() *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		ctx:       ctx,
		ctxCancel: cancel,
		trigger:   make(chan struct{}, 1),
		httpClient: &http.Client{
			Timeout: time.Second,
		},
	}
}

func TestNeedFastRetry(t *testing.T) {
	s := newTestService()

	require.False(t, s.needFastRetry(), "no providers, no keys")

	s.providerStore.Store(&[]rootModel.RootJwt{{JwkUrl: "http://jwk.local"}})
	require.True(t, s.needFastRetry(), "providers set, no keys")

	s.setItems(&keySet{all: map[string]*Item{"kid": {}}})
	require.False(t, s.needFastRetry(), "keys present")
}

func TestWorker_RecoversViaSignalAfterServerUnavailable(t *testing.T) {
	kid := "test-kid"

	var available atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !available.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(newTestItemSet(t, kid))
	}))
	defer srv.Close()

	s := newTestService()
	defer s.ctxCancel()

	go s.worker()

	// Имитируем приход конфига при недоступном JWK-сервере.
	s.SetProviders([]rootModel.RootJwt{{JwkUrl: srv.URL}})

	require.Never(t, func() bool {
		_, alg := s.GetPublicKey("", kid)
		return alg != ""
	}, 300*time.Millisecond, 30*time.Millisecond, "keys must stay empty while server is down")

	// Сервер поднялся; сигналим воркеру (как это делает SetProviders).
	available.Store(true)
	s.signal()

	require.Eventually(t, func() bool {
		pubKey, alg := s.GetPublicKey("", kid)
		return pubKey != nil && alg == "RS256"
	}, time.Second, 20*time.Millisecond, "keys must load promptly after recovery, without waiting for ScrapeInterval")
}

func TestLoad_KeepsKeysOfFailedUrl(t *testing.T) {
	kidA, kidB := "kid-a", "kid-b"
	setA, setB := newTestItemSet(t, kidA), newTestItemSet(t, kidB)

	var bDown atomic.Bool
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(setA)
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if bDown.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(setB)
	}))
	defer srvB.Close()

	s := newTestService()
	defer s.ctxCancel()
	s.providerStore.Store(&[]rootModel.RootJwt{{JwkUrl: srvA.URL}, {JwkUrl: srvB.URL}})

	s.load()
	require.Len(t, s.getItems().all, 2)
	require.False(t, s.needFastRetry())

	bDown.Store(true)
	s.load()
	pubKey, _ := s.GetPublicKey("", kidB)
	require.NotNil(t, pubKey, "keys of a failed url must survive")
	require.True(t, s.needFastRetry(), "failed url must be retried soon")

	// URL убрали из конфига — его ключи уходят, даже если он недоступен.
	s.providerStore.Store(&[]rootModel.RootJwt{{JwkUrl: srvA.URL}})
	s.load()
	pubKey, _ = s.GetPublicKey("", kidB)
	require.Nil(t, pubKey)
	require.False(t, s.needFastRetry())

	s.providerStore.Store(&[]rootModel.RootJwt{})
	s.load()
	require.Empty(t, s.getItems().all, "no providers left — no keys left")
}

func TestLoad_KeysByProvider(t *testing.T) {
	kidA, kidB := "kid-a", "kid-b"
	setA, setB := newTestItemSet(t, kidA), newTestItemSet(t, kidB)

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(setA)
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(setB)
	}))
	defer srvB.Close()

	s := newTestService()
	defer s.ctxCancel()
	s.providerStore.Store(&[]rootModel.RootJwt{
		{Name: "corp", JwkUrl: srvA.URL},
		{Name: "clients", JwkUrl: srvB.URL},
		{JwkUrl: srvB.URL}, // без имени: только в общем индексе
	})

	s.load()

	pubKey, _ := s.GetPublicKey("corp", kidA)
	require.NotNil(t, pubKey)
	pubKey, _ = s.GetPublicKey("corp", kidB)
	require.Nil(t, pubKey, "a provider must not expose keys of another one")
	pubKey, _ = s.GetPublicKey("clients", kidB)
	require.NotNil(t, pubKey)
	pubKey, _ = s.GetPublicKey("unknown", kidA)
	require.Nil(t, pubKey)

	// без провайдера — любой ключ
	pubKey, _ = s.GetPublicKey("", kidA)
	require.NotNil(t, pubKey)
	pubKey, _ = s.GetPublicKey("", kidB)
	require.NotNil(t, pubKey)
	require.Len(t, s.getItems().byProvider, 2)
}

// fakeIdP is an OpenID provider with one realm, "main", that can share its
// keys through the root JWKS: then its discovery points jwks_uri at the root
// set, otherwise at its own one and the root set is empty.
type fakeIdP struct {
	server        *httptest.Server
	issuer        string
	keys          atomic.Pointer[ItemSet]
	shared        atomic.Bool
	discoveryHits atomic.Int32
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()

	idp := &fakeIdP{}
	idp.keys.Store(new(newTestItemSet(t, "main-kid")))
	// The cache of the oidc service is shared: an issuer per test keeps the
	// tests apart.
	idp.issuer = "https://id.example/" + t.Name() + "/realms/main"
	idp.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/main/.well-known/openid-configuration":
			idp.discoveryHits.Add(1)
			jwksUri := idp.issuer + "/.well-known/jwks.json"
			if idp.shared.Load() {
				jwksUri = idp.server.URL + "/.well-known/jwks.json"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": idp.issuer, "jwks_uri": jwksUri})
		case "/realms/main/.well-known/jwks.json":
			_ = json.NewEncoder(w).Encode(idp.keys.Load())
		case "/.well-known/jwks.json":
			if idp.shared.Load() {
				_ = json.NewEncoder(w).Encode(idp.keys.Load())
				return
			}
			_ = json.NewEncoder(w).Encode(ItemSet{Keys: []*Item{}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *fakeIdP) provider() rootModel.RootJwt {
	return rootModel.RootJwt{Name: "id", Issuer: idp.issuer, InternalUrl: idp.server.URL + "/realms/main"}
}

func TestLoad_FollowsMovedJwksUri(t *testing.T) {
	idp := newFakeIdP(t)
	idp.shared.Store(true)

	s := newTestService()
	defer s.ctxCancel()
	s.SetProviders([]rootModel.RootJwt{idp.provider()})

	s.load()
	pubKey, _ := s.GetPublicKey("id", "main-kid")
	require.NotNil(t, pubKey, "shared: the keys come from the root set")

	// The realm stops sharing: the cached document still points at the root
	// set, which is empty now.
	idp.shared.Store(false)
	s.load()
	pubKey, _ = s.GetPublicKey("id", "main-kid")
	require.NotNil(t, pubKey, "the keys must survive the move of jwks_uri")
	require.False(t, s.needFastRetry(), "the keys were found at the new address")
	require.EqualValues(t, 2, idp.discoveryHits.Load(), "the empty set must make the gateway read discovery again")
}

func TestLoad_EmptySetKeepsPreviousKeys(t *testing.T) {
	kid := "kid-a"
	set := newTestItemSet(t, kid)
	var empty atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if empty.Load() {
			_ = json.NewEncoder(w).Encode(ItemSet{Keys: []*Item{}})
			return
		}
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer srv.Close()

	s := newTestService()
	defer s.ctxCancel()
	s.providerStore.Store(&[]rootModel.RootJwt{{Name: "jwts", JwkUrl: srv.URL}})

	s.load()
	empty.Store(true)
	s.load()

	pubKey, _ := s.GetPublicKey("jwts", kid)
	require.NotNil(t, pubKey, "an empty set must not revoke working keys")
	require.True(t, s.needFastRetry(), "an empty set must be polled again soon")

	empty.Store(false)
	s.load()
	require.False(t, s.needFastRetry())
}

func TestLoad_FullRotationIsTaken(t *testing.T) {
	var current atomic.Pointer[ItemSet]
	current.Store(new(newTestItemSet(t, "old-kid")))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(current.Load())
	}))
	defer srv.Close()

	s := newTestService()
	defer s.ctxCancel()
	s.providerStore.Store(&[]rootModel.RootJwt{{JwkUrl: srv.URL}})

	s.load()
	current.Store(new(newTestItemSet(t, "new-kid")))
	s.load()

	pubKey, _ := s.GetPublicKey("", "new-kid")
	require.NotNil(t, pubKey)
	pubKey, _ = s.GetPublicKey("", "old-kid")
	require.Nil(t, pubKey, "a key the provider no longer publishes must go")
	require.False(t, s.needFastRetry())
}

func TestSetProviders_ReaddedProviderReadsDiscoveryAgain(t *testing.T) {
	idp := newFakeIdP(t)

	s := newTestService()
	defer s.ctxCancel()
	s.SetProviders([]rootModel.RootJwt{idp.provider()})
	s.load()
	require.EqualValues(t, 1, idp.discoveryHits.Load())

	s.SetProviders([]rootModel.RootJwt{idp.provider()})
	s.load()
	require.EqualValues(t, 1, idp.discoveryHits.Load(), "a provider that stays keeps its cached document")

	s.SetProviders(nil)
	s.SetProviders([]rootModel.RootJwt{idp.provider()})
	s.load()
	require.EqualValues(t, 2, idp.discoveryHits.Load(), "a provider added again starts from a fresh document")
}

func newTestItemSet(t *testing.T, kid string) ItemSet {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	eBytes := big.NewInt(int64(priv.E)).Bytes()

	return ItemSet{
		Keys: []*Item{
			{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: kid,
				N:   base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}
}
