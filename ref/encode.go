package ref

import (
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Ref is a reference to a field or an index in a protobuf message.
// It holds single value so we don'y need variant encoding.
type Ref struct {
	v []byte
}

func (x Ref) Value() []byte {
	return x.v
}

func Field(v protoreflect.FieldNumber) Ref {
	return Ref{EncodeInt(uint64(v))}
}

func Index(v int) Ref {
	return SignedKey(v)
}

func StringKey(v string) Ref {
	return Ref{EncodeString(v)}
}

func Fixed32Key[T uint32 | int32](v T) Ref {
	return Ref{EncodeFixed32(uint32(v))}
}

func Fixed64Key[T uint64 | int64](v T) Ref {
	return Ref{EncodeFixed64(uint64(v))}
}

func SignedKey[T constraints.Signed](v T) Ref {
	return Ref{EncodeInt(protowire.EncodeZigZag(int64(v)))}
}
