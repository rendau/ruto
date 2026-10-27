package compress

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decode(t *testing.T, coding string, b []byte) string {
	t.Helper()

	var r io.Reader
	switch coding {
	case "br":
		r = brotli.NewReader(bytes.NewReader(b))
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(b))
		require.NoError(t, err)
		r = zr
	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(b))
		require.NoError(t, err)
		defer zr.Close()
		r = zr
	default:
		t.Fatalf("unknown coding %q", coding)
	}

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestRoundTripAndReuse(t *testing.T) {
	payload := strings.Repeat(`{"id":1,"name":"item","tags":["a","b"]}`, 200)

	for _, coding := range []string{"br", "zstd", "gzip"} {
		t.Run(coding, func(t *testing.T) {
			// Twice: the second run takes the encoder the first one pooled.
			for range 2 {
				var buf bytes.Buffer
				enc, ok := NewEncoder(coding, &buf)
				require.True(t, ok)

				_, err := io.WriteString(enc, payload[:100])
				require.NoError(t, err)
				require.NoError(t, enc.Flush())
				flushed := buf.Len()
				assert.Positive(t, flushed, "flush must push bytes through")

				_, err = io.WriteString(enc, payload[100:])
				require.NoError(t, err)
				require.NoError(t, enc.Close())

				assert.Less(t, buf.Len(), len(payload)/4)
				assert.Equal(t, payload, decode(t, coding, buf.Bytes()))
			}
		})
	}
}

func TestNewEncoderUnknown(t *testing.T) {
	_, ok := NewEncoder("deflate", io.Discard)
	assert.False(t, ok)
	assert.True(t, Supported("br"))
	assert.False(t, Supported("deflate"))
}
