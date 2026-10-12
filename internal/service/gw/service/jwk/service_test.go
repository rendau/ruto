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

	require.False(t, s.needFastRetry(), "no urls, no keys")

	s.urlStore.Store(&[]string{"http://jwk.local"})
	require.True(t, s.needFastRetry(), "urls set, no keys")

	s.setItems(map[string]*Item{"kid": {}})
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
	s.SetUrls([]string{srv.URL})

	require.Never(t, func() bool {
		_, alg := s.GetPublicKey(kid)
		return alg != ""
	}, 300*time.Millisecond, 30*time.Millisecond, "keys must stay empty while server is down")

	// Сервер поднялся; сигналим воркеру (как это делает SetUrls).
	available.Store(true)
	s.signal()

	require.Eventually(t, func() bool {
		pubKey, alg := s.GetPublicKey(kid)
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
	s.urlStore.Store(&[]string{srvA.URL, srvB.URL})

	s.load()
	require.Len(t, s.getItems(), 2)
	require.False(t, s.needFastRetry())

	bDown.Store(true)
	s.load()
	pubKey, _ := s.GetPublicKey(kidB)
	require.NotNil(t, pubKey, "keys of a failed url must survive")
	require.True(t, s.needFastRetry(), "failed url must be retried soon")

	// URL убрали из конфига — его ключи уходят, даже если он недоступен.
	s.urlStore.Store(&[]string{srvA.URL})
	s.load()
	pubKey, _ = s.GetPublicKey(kidB)
	require.Nil(t, pubKey)
	require.False(t, s.needFastRetry())

	s.urlStore.Store(&[]string{})
	s.load()
	require.Empty(t, s.getItems(), "no urls left — no keys left")
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
