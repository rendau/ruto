package model

import (
	"slices"
	"strings"

	"github.com/rendau/ruto/internal/constant"
)

// Compression holds response-compression configuration. It is defined on Root,
// App and Endpoint and flows down via Merge (see merge.go): a field the child
// leaves empty is inherited from the parent. Root fills its empty fields with
// the defaults (see Default), so an endpoint always ends up with every field
// set.
type Compression struct {
	// Backend tells what the gateway asks the backend for: "identity" strips
	// the client's Accept-Encoding so the body comes uncompressed and the
	// gateway compresses it itself; "client" passes the header on and a backend
	// that compresses answers compressed, which the gateway sends as is.
	Backend string `json:"backend"` // "" | "identity" | "client"
	// Response switches compressing the body sent to the client.
	Response string `json:"response"` // "" | "on" | "off"
	// Algorithms are the encodings offered, in order of preference: the client
	// gets the first one its Accept-Encoding allows.
	Algorithms []string `json:"algorithms"` // "br" | "zstd" | "gzip"
	// MinSize is the body size in bytes under which a response is sent as is.
	MinSize int `json:"min_size"` // 0 => inherit
	// ContentTypes lists the media types worth compressing. "type/*" matches a
	// whole type.
	ContentTypes []string `json:"content_types"`
}

// Default is the configuration a root without explicit values gets.
func Default() Compression {
	return Compression{
		Backend:      constant.CompressionBackendIdentity,
		Response:     constant.CompressionResponseOn,
		Algorithms:   slices.Clone(constant.DefaultCompressionAlgorithms),
		MinSize:      constant.DefaultCompressionMinSize,
		ContentTypes: slices.Clone(constant.DefaultCompressionContentTypes),
	}
}

// BackendIdentity reports whether the backend is asked for an uncompressed
// body. Unset falls back to the default.
func (m *Compression) BackendIdentity() bool {
	return m.Backend != constant.CompressionBackendClient
}

// ResponseEnabled reports whether the body sent to the client is compressed.
// Unset falls back to the default.
func (m *Compression) ResponseEnabled() bool {
	return m.Response != constant.CompressionResponseOff
}

func (m *Compression) AlgorithmsOrDefault() []string {
	if len(m.Algorithms) == 0 {
		return constant.DefaultCompressionAlgorithms
	}
	return m.Algorithms
}

func (m *Compression) MinSizeOrDefault() int {
	if m.MinSize <= 0 {
		return constant.DefaultCompressionMinSize
	}
	return m.MinSize
}

func (m *Compression) ContentTypesOrDefault() []string {
	if len(m.ContentTypes) == 0 {
		return constant.DefaultCompressionContentTypes
	}
	return m.ContentTypes
}

// MatchesContentType reports whether a Content-Type header value names a
// media type worth compressing. Parameters (charset) are ignored; "type/*"
// entries match the whole type; a structured suffix ("application/ld+json")
// also matches the plain type ("application/json").
func (m *Compression) MatchesContentType(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" {
		return false
	}

	candidates := []string{mediaType}
	mainType, subType, _ := strings.Cut(mediaType, "/")
	candidates = append(candidates, mainType+"/*")
	if _, suffix, found := strings.Cut(subType, "+"); found {
		candidates = append(candidates, "application/"+suffix)
	}

	return slices.ContainsFunc(m.ContentTypesOrDefault(), func(entry string) bool {
		return slices.Contains(candidates, entry)
	})
}
