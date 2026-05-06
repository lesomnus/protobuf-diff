package ref

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func DecodeField(data []byte) (protoreflect.FieldNumber, error) {
	v := DecodeInt(data)
	return protoreflect.FieldNumber(v), nil
}

func DecodeIndex(data []byte) (int, error) {
	v := DecodeInt(data)
	return int(protowire.DecodeZigZag(v)), nil
}
