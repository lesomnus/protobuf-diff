package dpb

import (
	"math"

	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func NewDelta(es ...*Entry) *Delta {
	return Delta_builder{Entries: es}.Build()
}

func Index(i uint32) []byte {
	return protowire.AppendVarint(nil, uint64(i))
}

func Double(v float64) []byte {
	return protowire.AppendFixed64(nil, math.Float64bits(v))
}

func Float(v float32) []byte {
	return protowire.AppendFixed32(nil, math.Float32bits(v))
}

func Int[T constraints.Integer](v T) []byte {
	return protowire.AppendVarint(nil, uint64(v))
}

func Bool(v bool) []byte {
	var i uint64
	if v {
		i = 1
	}
	return protowire.AppendVarint(nil, i)
}

func String(v string) []byte {
	return []byte(v)
}

func Message(v proto.Message) []byte {
	b, _ := proto.Marshal(v)
	return b
}

func Bytes(v []byte) []byte {
	return v
}

func Enum[T constraints.Integer](v T) []byte {
	w := protoreflect.EnumNumber(v)
	return protowire.AppendVarint(nil, uint64(w))
}

func Fixed[T constraints.Integer](v T) []byte {
	return protowire.AppendFixed64(nil, uint64(v))
}

func Signed[T constraints.Signed](v T) []byte {
	w := protowire.EncodeZigZag(int64(v))
	return protowire.AppendVarint(nil, uint64(w))
}
