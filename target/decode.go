package target

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func DecodeFieldNumbers(data []byte) ([]protoreflect.FieldNumber, error) {
	vs := []protoreflect.FieldNumber{}
	for len(data) > 0 {
		v, n := protowire.ConsumeVarint(data)
		if n < 0 {
			return nil, fmt.Errorf("invalid varint encoding: %w", protowire.ParseError(n))
		}

		vs = append(vs, protoreflect.FieldNumber(v))
		data = data[n:]
	}
	return vs, nil
}

func DecodeIndices(data []byte) ([]int, error) {
	vs := []int{}
	for len(data) > 0 {
		v, n := protowire.ConsumeVarint(data)
		if n < 0 {
			return nil, fmt.Errorf("invalid varint encoding: %w", protowire.ParseError(n))
		}

		w := protowire.DecodeZigZag(v)
		vs = append(vs, int(w))
		data = data[n:]
	}
	return vs, nil
}

func DecodeKeys(data []byte, kind protoreflect.Kind) ([]protoreflect.MapKey, error) {
	var keys []protoreflect.MapKey
	for len(data) > 0 {
		k, n, err := consumeKey(data, kind)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
		data = data[n:]
	}
	return keys, nil
}

func consumeKey(data []byte, kind protoreflect.Kind) (protoreflect.MapKey, int, error) {
	var v protoreflect.Value
	var n int

	switch kind {
	case protoreflect.BoolKind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfBool(u != 0)
		n = m

	case protoreflect.Int32Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfInt32(int32(u))
		n = m

	case protoreflect.Sint32Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfInt32(int32(protowire.DecodeZigZag(u)))
		n = m

	case protoreflect.Uint32Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfUint32(uint32(u))
		n = m

	case protoreflect.Int64Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfInt64(int64(u))
		n = m

	case protoreflect.Sint64Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfInt64(protowire.DecodeZigZag(u))
		n = m

	case protoreflect.Uint64Kind:
		u, m := protowire.ConsumeVarint(data)
		v = protoreflect.ValueOfUint64(u)
		n = m

	case protoreflect.Sfixed32Kind:
		u, m := protowire.ConsumeFixed32(data)
		v = protoreflect.ValueOfInt32(int32(u))
		n = m

	case protoreflect.Fixed32Kind:
		u, m := protowire.ConsumeFixed32(data)
		v = protoreflect.ValueOfUint32(u)
		n = m

	case protoreflect.Sfixed64Kind:
		u, m := protowire.ConsumeFixed64(data)
		v = protoreflect.ValueOfInt64(int64(u))
		n = m

	case protoreflect.Fixed64Kind:
		u, m := protowire.ConsumeFixed64(data)
		v = protoreflect.ValueOfUint64(u)
		n = m

	case protoreflect.StringKind:
		u, m := protowire.ConsumeString(data)
		v = protoreflect.ValueOfString(u)
		n = m

	default:
		return protoreflect.MapKey{}, 0, fmt.Errorf("unsupported map key kind: %v", kind)
	}

	if n < 0 {
		return protoreflect.MapKey{}, n, fmt.Errorf("invalid %v key: %w", kind, protowire.ParseError(n))
	}
	return v.MapKey(), n, nil
}
