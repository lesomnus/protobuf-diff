package target

import (
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Targets struct {
	v []byte
}

func (x Targets) Value() []byte {
	return x.v
}

func Fields(vs ...protoreflect.FieldNumber) Targets {
	bs := []byte{}
	for _, v := range vs {
		bs = protowire.AppendVarint(bs, uint64(v))
	}
	return Targets{bs}
}

func Indices(vs ...int) Targets {
	bs := []byte{}
	for _, v := range vs {
		bs = protowire.AppendVarint(bs, protowire.EncodeZigZag(int64(v)))
	}
	return Targets{bs}
}

func StringKeys(vs ...string) Targets {
	bs := []byte{}
	for _, v := range vs {
		bs = protowire.AppendString(bs, v)
	}
	return Targets{bs}
}

func FixedKeys[T constraints.Integer](vs ...T) Targets {
	bs := []byte{}
	for _, v := range vs {
		bs = protowire.AppendFixed64(bs, uint64(v))
	}
	return Targets{bs}
}

func SignedKeys[T constraints.Signed](vs ...T) Targets {
	bs := []byte{}
	for _, v := range vs {
		bs = protowire.AppendVarint(bs, protowire.EncodeZigZag(int64(v)))
	}
	return Targets{bs}
}
