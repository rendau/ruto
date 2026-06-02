package grpc

import (
	"context"
	"net/http"

	"google.golang.org/grpc/metadata"
)

func metadataFromContext(ctx context.Context) metadata.MD {
	md, _ := metadata.FromIncomingContext(ctx)
	if md == nil {
		md = metadata.New(map[string]string{})
	}
	return md
}

func metadataFirstValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func headersFromMetadata(md metadata.MD) http.Header {
	return http.Header(md)
}
