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

	"github.com/goccy/go-json"
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
