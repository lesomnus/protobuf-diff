package patchproto

import (
	"strconv"

	"golang.org/x/exp/constraints"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func cast(v protoreflect.Value, from, to protoreflect.Kind) (protoreflect.Value, error) {
	if from == to {
		return v, nil
	}

	switch from {
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		u := v.Float()
		return castNumber(u, from, to)

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		u := v.Uint()
		return castNumber(u, from, to)

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		u := v.Int()
		return castNumber(u, from, to)

	case protoreflect.BoolKind:
		u := 0
		if v.Bool() {
			u = 1
		}
		return castNumber(u, from, to)

	case protoreflect.StringKind:
		switch to {
		case protoreflect.BytesKind:
			return protoreflect.ValueOfBytes([]byte(v.String())), nil
		case protoreflect.Int32Kind, protoreflect.Int64Kind,
			protoreflect.Sint32Kind, protoreflect.Sint64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
			u, err := strconv.ParseInt(v.String(), 10, 64)
			if err != nil {
				return protoreflect.Value{}, ErrInvalidCast{from, to}
			}
			return castNumber(u, from, to)
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
			protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
			u, err := strconv.ParseUint(v.String(), 10, 64)
			if err != nil {
				return protoreflect.Value{}, ErrInvalidCast{from, to}
			}
			return castNumber(u, from, to)
		case protoreflect.FloatKind:
			u, err := strconv.ParseFloat(v.String(), 32)
			if err != nil {
				return protoreflect.Value{}, ErrInvalidCast{from, to}
			}
			return castNumber(float32(u), from, to)
		case protoreflect.DoubleKind:
			u, err := strconv.ParseFloat(v.String(), 64)
			if err != nil {
				return protoreflect.Value{}, ErrInvalidCast{from, to}
			}
			return castNumber(u, from, to)
		}

	case protoreflect.BytesKind:
		if to == protoreflect.StringKind {
			return protoreflect.ValueOfString(string(v.Bytes())), nil
		}
	}

	return protoreflect.Value{}, ErrInvalidCast{from, to}
}

func castNumber[T constraints.Integer | constraints.Float](u T, from, to protoreflect.Kind) (protoreflect.Value, error) {
	w := protoreflect.Value{}
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
	case protoreflect.StringKind:
		if u < 0 {
			w = protoreflect.ValueOfString(strconv.FormatInt(int64(u), 10))
		} else {
			w = protoreflect.ValueOfString(strconv.FormatUint(uint64(u), 10))
		}
	default:
		return w, ErrInvalidCast{from, to}
	}

	return w, nil
}
