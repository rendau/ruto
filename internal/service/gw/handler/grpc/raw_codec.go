package grpc

import "fmt"

type rawFrame struct {
	Payload []byte
}

type rawCodec struct{}

func (rawCodec) Name() string { return "ruto-gw-raw" }

func (rawCodec) Marshal(v any) ([]byte, error) {
	switch item := v.(type) {
	case *rawFrame:
		if item == nil {
			return nil, nil
		}
		return item.Payload, nil
	case rawFrame:
		return item.Payload, nil
	default:
		return nil, fmt.Errorf("unsupported raw frame type: %T", v)
	}
}

func (rawCodec) Unmarshal(data []byte, v any) error {
	frame, ok := v.(*rawFrame)
	if !ok {
		return fmt.Errorf("unsupported raw frame type: %T", v)
	}
	if frame == nil {
		return nil
	}
	frame.Payload = append(frame.Payload[:0], data...)
	return nil
}
