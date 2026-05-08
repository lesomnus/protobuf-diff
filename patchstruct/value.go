package patchstruct

import (
	"errors"
	"fmt"
	"math"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
)

func decodeValue(b []byte, t reflect.Type) (reflect.Value, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	r := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.Bool:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return reflect.Value{}, errors.New("invalid varint")
		}
		r.SetBool(v != 0)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return reflect.Value{}, errors.New("invalid varint")
		}
		r.SetInt(int64(v))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return reflect.Value{}, errors.New("invalid varint")
		}
		r.SetUint(v)

	case reflect.Float32:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return reflect.Value{}, errors.New("invalid fixed32")
		}
		r.SetFloat(float64(math.Float32frombits(v)))

	case reflect.Float64:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return reflect.Value{}, errors.New("invalid fixed64")
		}
		r.SetFloat(math.Float64frombits(v))

	case reflect.String:
		r.SetString(string(b))

	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			cp := make([]byte, len(b))
			copy(cp, b)
			r.Set(reflect.ValueOf(cp).Convert(t))
		} else {
			return reflect.Value{}, fmt.Errorf("unimplemented: %v", t)
		}

	default:
		return reflect.Value{}, fmt.Errorf("unimplemented: %v", t)
	}

	return r, nil
}

// setField sets dst to src, wrapping src in a pointer if dst is a pointer type.
// src must be the base (non-pointer) value.
func setField(dst, src reflect.Value) {
	if dst.Kind() == reflect.Pointer {
		ptr := reflect.New(dst.Type().Elem())
		ptr.Elem().Set(src)
		dst.Set(ptr)
	} else {
		dst.Set(src)
	}
}
