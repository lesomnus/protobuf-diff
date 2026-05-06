package ref

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func EncodeInt(v uint64) []byte {
	switch {
	case v < 0x100:
		return []byte{byte(v)}
	case v < 0x10000:
		return []byte{byte(v), byte(v >> 8)}
	case v < 0x1000000:
		return []byte{byte(v), byte(v >> 8), byte(v >> 16)}
	default:
		return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
	}
}

func DecodeInt(data []byte) uint64 {
	var v uint64
	for i, b := range data {
		v |= uint64(b) << (8 * i)
	}
	return v
}

func EncodeFixed32(v uint32) []byte {
	return []byte{
		byte(v),
		byte(v >> 8),
		byte(v >> 16),
		byte(v >> 24),
	}
}

func EncodeFixed64(v uint64) []byte {
	return []byte{
		byte(v),
		byte(v >> 8),
		byte(v >> 16),
		byte(v >> 24),
		byte(v >> 32),
		byte(v >> 40),
		byte(v >> 48),
		byte(v >> 56),
	}
}

func EncodeString(s string) []byte {
	return []byte(s)
}

func DecodeString(data []byte) string {
	return string(data)
}

func DecodeKey(data []byte, kind protoreflect.Kind) (protoreflect.MapKey, error) {
	var v protoreflect.Value

	switch kind {
	case protoreflect.BoolKind:
		u := 0
		if len(data) > 0 {
			u = int(data[0])
		}
		v = protoreflect.ValueOfBool(u != 0)

	case protoreflect.Int32Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt32(int32(u))

	case protoreflect.Sint32Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt32(int32(protowire.DecodeZigZag(u)))

	case protoreflect.Uint32Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfUint32(uint32(u))

	case protoreflect.Int64Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt64(int64(u))

	case protoreflect.Sint64Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt64(int64(protowire.DecodeZigZag(u)))

	case protoreflect.Uint64Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfUint64(u)

	case protoreflect.Sfixed32Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt32(int32(u))

	case protoreflect.Fixed32Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfUint32(uint32(u))

	case protoreflect.Sfixed64Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfInt64(int64(u))

	case protoreflect.Fixed64Kind:
		u := DecodeInt(data)
		v = protoreflect.ValueOfUint64(u)

	case protoreflect.StringKind:
		u := DecodeString(data)
		v = protoreflect.ValueOfString(u)

	default:
		return protoreflect.MapKey{}, fmt.Errorf("unsupported map key kind: %v", kind)
	}

	return v.MapKey(), nil
}
