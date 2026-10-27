// Package compress hands out pooled encoders for the content codings the
// gateway offers. An encoder is heavy to build (brotli and zstd keep
// megabytes of state), so Close returns it to its pool instead of dropping it.
package compress

import (
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"

	"github.com/rendau/ruto/internal/constant"
)

// Levels are fixed on the fast side: a response is compressed once and sent,
// so a higher level costs CPU on every request for a few percent of size.
const (
	brotliQuality = 5
	brotliLGWin   = 18 // 256KB window
	gzipLevel     = 5
	zstdWindow    = 1 << 19 // 512KB
)

// Encoder writes compressed bytes to the writer it was opened on. Flush pushes
// what was written so far through, for streamed responses; Close ends the
// stream and returns the encoder to its pool.
type Encoder interface {
	io.WriteCloser
	Flush() error
}

type resettable interface {
	Encoder
	Reset(w io.Writer)
}

var pools = map[string]*sync.Pool{
	constant.CompressionAlgorithmBr: {New: func() any {
		return brotli.NewWriterOptions(io.Discard, brotli.WriterOptions{Quality: brotliQuality, LGWin: brotliLGWin})
	}},
	constant.CompressionAlgorithmGzip: {New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzipLevel)
		return w
	}},
	constant.CompressionAlgorithmZstd: {New: func() any {
		w, _ := zstd.NewWriter(io.Discard,
			zstd.WithEncoderLevel(zstd.SpeedDefault),
			zstd.WithEncoderConcurrency(1),
			zstd.WithWindowSize(zstdWindow),
			zstd.WithLowerEncoderMem(true),
		)
		return w
	}},
}

// Supported reports whether the gateway can produce the coding.
func Supported(coding string) bool {
	_, ok := pools[coding]
	return ok
}

// NewEncoder opens an encoder for the coding on w. ok is false for a coding
// the gateway cannot produce.
func NewEncoder(coding string, w io.Writer) (_ Encoder, ok bool) {
	pool, ok := pools[coding]
	if !ok {
		return nil, false
	}

	enc := pool.Get().(resettable)
	enc.Reset(w)

	return &pooledEncoder{resettable: enc, pool: pool}, true
}

type pooledEncoder struct {
	resettable
	pool *sync.Pool
}

func (e *pooledEncoder) Close() error {
	err := e.resettable.Close()
	// Reset onto io.Discard so the pooled encoder does not keep the response
	// writer alive.
	e.resettable.Reset(io.Discard)
	e.pool.Put(e.resettable)
	return err
}
