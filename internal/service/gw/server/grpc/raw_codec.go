package grpc

import (
	"fmt"
	"reflect"
)

type rawCodec struct{}

func (rawCodec) Name() string { return "ruto-gw-raw" }

func (rawCodec) Marshal(v any) ([]byte, error) {
	payload, err := getPayloadBytes(v)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (rawCodec) Unmarshal(data []byte, v any) error {
	if v == nil {
		return nil
	}
	return setPayloadBytes(v, data)
}

func getPayloadBytes(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported raw frame type: %T", v)
	}

	payloadField := rv.FieldByName("Payload")
	if !payloadField.IsValid() || payloadField.Kind() != reflect.Slice || payloadField.Type().Elem().Kind() != reflect.Uint8 {
		return nil, fmt.Errorf("unsupported raw frame type: %T", v)
	}

	if payloadField.IsNil() {
		return nil, nil
	}

	result := make([]byte, payloadField.Len())
	reflect.Copy(reflect.ValueOf(result), payloadField)
	return result, nil
}

func setPayloadBytes(v any, data []byte) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("unsupported raw frame type: %T", v)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("unsupported raw frame type: %T", v)
	}

	payloadField := rv.FieldByName("Payload")
	if !payloadField.IsValid() || !payloadField.CanSet() {
		return fmt.Errorf("unsupported raw frame type: %T", v)
	}
	if payloadField.Kind() != reflect.Slice || payloadField.Type().Elem().Kind() != reflect.Uint8 {
		return fmt.Errorf("unsupported raw frame type: %T", v)
	}

	next := make([]byte, len(data))
	copy(next, data)
	payloadField.SetBytes(next)
	return nil
}
