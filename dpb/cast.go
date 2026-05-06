package dpb

import (
	"golang.org/x/exp/constraints"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func cast(v protoreflect.Value, from, to protoreflect.Kind) (w protoreflect.Value, ok bool) {
	if from == to {
		return v, true
	}

	switch from {
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		u := v.Float()
		return castNumber(u, to)

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		u := v.Uint()
		return castNumber(u, to)

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		u := v.Int()
		return castNumber(u, to)

	case protoreflect.BoolKind:
		u := 0
		if v.Bool() {
			u = 1
		}
		return castNumber(u, to)

	case protoreflect.StringKind:
		if to == protoreflect.BytesKind {
			w = protoreflect.ValueOfBytes([]byte(v.String()))
			ok = true
		}

	case protoreflect.BytesKind:
		if to == protoreflect.StringKind {
			w = protoreflect.ValueOfString(string(v.Bytes()))
			ok = true
		}
	}

	return
}

func castNumber[T constraints.Integer | constraints.Float](u T, to protoreflect.Kind) (w protoreflect.Value, ok bool) {
	switch to {
	case protoreflect.FloatKind:
		w = protoreflect.ValueOfFloat32(float32(u))
	case protoreflect.DoubleKind:
		w = protoreflect.ValueOfFloat64(float64(u))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		w = protoreflect.ValueOfInt32(int32(u))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		w = protoreflect.ValueOfInt64(int64(u))
	case protoreflect.EnumKind:
		w = protoreflect.ValueOfEnum(protoreflect.EnumNumber(u))
	case protoreflect.BoolKind:
		w = protoreflect.ValueOfBool(u != 0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		w = protoreflect.ValueOfUint32(uint32(u))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		w = protoreflect.ValueOfUint64(uint64(u))
	default:
		return protoreflect.Value{}, false
	}

	ok = true
	return
}
