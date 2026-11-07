// Package httpx builds the http clients the services call other systems with.
// Every client gets the same guard rails: a dial and a TLS handshake timeout,
// a cap on connections per host and tolerance for the self-signed certificates
// of internal services. Zero config fields take the defaults below.
package httpx

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

const (
	DefaultDialTimeout         = 2 * time.Second
	DefaultTLSHandshakeTimeout = 2 * time.Second
	DefaultMaxConnsPerHost     = 20
)

type Config struct {
	// Timeout bounds a whole request; leave it zero for streaming responses
	// and bound them with the context instead.
	Timeout             time.Duration
	DialTimeout         time.Duration
	TLSHandshakeTimeout time.Duration
	MaxConnsPerHost     int
	// DisableKeepAlives is for rare calls: the server would close an idle
	// connection before it is needed again.
	DisableKeepAlives bool
}

func New(conf Config) *http.Client {
	return &http.Client{
		Timeout:   conf.Timeout,
		Transport: NewTransport(conf),
	}
}

func NewTransport(conf Config) *http.Transport {
	dialTimeout := conf.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = DefaultDialTimeout
	}
	tlsTimeout := conf.TLSHandshakeTimeout
	if tlsTimeout <= 0 {
		tlsTimeout = DefaultTLSHandshakeTimeout
	}
	maxConns := conf.MaxConnsPerHost
	if maxConns <= 0 {
		maxConns = DefaultMaxConnsPerHost
	}

	return &http.Transport{
		DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
		TLSHandshakeTimeout: tlsTimeout,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // internal services use self-signed certificates
		MaxConnsPerHost:     maxConns,
		MaxIdleConnsPerHost: maxConns,
		DisableKeepAlives:   conf.DisableKeepAlives,
	}
}
