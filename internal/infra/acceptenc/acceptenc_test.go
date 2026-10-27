package acceptenc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccepts(t *testing.T) {
	tests := []struct {
		header string
		coding string
		want   bool
	}{
		{header: "gzip, deflate, br", coding: "br", want: true},
		{header: "gzip, deflate, br", coding: "gzip", want: true},
		{header: "Gzip", coding: "gzip", want: true},
		{header: "br;q=0.5", coding: "br", want: true},
		{header: "br; q=0", coding: "br", want: false},
		{header: "br;q=0.0, gzip", coding: "br", want: false},
		{header: "br;q=bad", coding: "br", want: false},
		{header: "x-br, gzip", coding: "br", want: false},
		{header: "", coding: "gzip", want: false},
		{header: "*", coding: "zstd", want: true},
		{header: "*;q=0", coding: "zstd", want: false},
		{header: "gzip, *;q=0.1", coding: "br", want: true},
		{header: "br;q=0, *", coding: "br", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.header+"/"+tt.coding, func(t *testing.T) {
			assert.Equal(t, tt.want, Accepts(tt.header, tt.coding))
		})
	}
}

func TestPick(t *testing.T) {
	offered := []string{"br", "zstd", "gzip"}

	tests := []struct {
		header string
		want   string
	}{
		{header: "gzip, deflate, br", want: "br"},
		{header: "gzip, deflate", want: "gzip"},
		{header: "zstd;q=0.9, gzip;q=1", want: "zstd"},
		{header: "br;q=0, gzip", want: "gzip"},
		{header: "identity", want: ""},
		{header: "", want: ""},
		{header: "*", want: "br"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.want, Pick(tt.header, offered))
		})
	}
}
