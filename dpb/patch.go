package dpb

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Patcher interface {
	Patch(v proto.Message, delta *Delta) error
}

func Patch(v proto.Message, delta *Delta) error {
	return PatchOption{}.Patch(v, delta)
}

func Patched[T proto.Message](v T, delta *Delta) (T, error) {
	v = proto.CloneOf(v)
	if err := Patch(v, delta); err != nil {
		var z T
		return z, err
	}
	return v, nil
}

type PatchOption struct{}

func (o PatchOption) Patch(v proto.Message, delta *Delta) error {
	return o.PatchField(v.ProtoReflect(), nil, delta)
}

func (o PatchOption) PatchField(v any, fd protoreflect.FieldDescriptor, delta *Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.patch(protoreflect.ValueOf(nil), v, fd, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}

	return nil
}

func (o PatchOption) patch(p protoreflect.Value, v any, fd protoreflect.FieldDescriptor, entry *Entry) error {
	segments := slices.Collect(entry.Path().Seq())

	var s any
	if len(entry.GetTargets()) == 0 {
		segments, s = segments[:len(segments)-1], segments[len(segments)-1]
	}

	v, fd, err := Navigate(v, fd, segments)
	if err != nil {
		return fmt.Errorf("navigate path: %w", err)
	}

	switch v := v.(type) {
	case protoreflect.Message:
		switch s := s.(type) {
		case string:
			fd := v.Descriptor().Fields().ByName(protoreflect.Name(s))
			if fd == nil {
				return nil
			}
			entry.AppendTargets(target.Fields(fd.Number()))

		case int:
			entry.AppendTargets(target.Fields(protoreflect.FieldNumber(s)))

		case uint:
			entry.AppendTargets(target.Fields(protoreflect.FieldNumber(s)))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type: %T", s)
		}
		return o.patchMessage(v, entry)

	case protoreflect.List:
		switch s := s.(type) {
		case int:
			entry.AppendTargets(target.Indices(s))

		case uint:
			entry.AppendTargets(target.Indices(int(s)))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type for list: %T", s)
		}
		return o.patchList(v, fd, entry)

	case protoreflect.Map:
		switch s := s.(type) {
		case string:
			entry.AppendTargets(target.StringKeys(s))

		case int:
			entry.AppendTargets(target.SignedKeys(s))

		case uint:
			entry.AppendTargets(target.UnsignedKeys(s))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type: %T", s)
		}
		return o.patchMap(v, fd, entry)

	default:
		return fmt.Errorf("unsupported value type: %T", v)
	}
}

// decodeValue decodes a field value from its wire-format bytes based on the field descriptor.
func decodeValue(b []byte, fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfBool(v != 0), nil

	case protoreflect.EnumKind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfEnum(protoreflect.EnumNumber(int32(v))), nil

	case protoreflect.Int32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt32(int32(v)), nil

	case protoreflect.Sint32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt32(int32(protowire.DecodeZigZag(v))), nil

	case protoreflect.Uint32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfUint32(uint32(v)), nil

	case protoreflect.Int64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt64(int64(v)), nil

	case protoreflect.Sint64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt64(protowire.DecodeZigZag(v)), nil

	case protoreflect.Uint64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfUint64(v), nil

	case protoreflect.Sfixed32Kind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfInt32(int32(v)), nil

	case protoreflect.Fixed32Kind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfUint32(v), nil

	case protoreflect.FloatKind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfFloat32(math.Float32frombits(v)), nil

	case protoreflect.Sfixed64Kind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfInt64(int64(v)), nil

	case protoreflect.Fixed64Kind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfUint64(v), nil

	case protoreflect.DoubleKind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfFloat64(math.Float64frombits(v)), nil

	case protoreflect.StringKind:
		return protoreflect.ValueOfString(string(b)), nil

	case protoreflect.BytesKind:
		cp := make([]byte, len(b))
		copy(cp, b)
		return protoreflect.ValueOfBytes(cp), nil

	case protoreflect.MessageKind, protoreflect.GroupKind:
		mt, err := protoregistry.GlobalTypes.FindMessageByName(fd.Message().FullName())
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("find message type %q: %w", fd.Message().FullName(), err)
		}

		m := mt.New()
		if err := proto.Unmarshal(b, m.Interface()); err != nil {
			return protoreflect.Value{}, fmt.Errorf("unmarshal message: %w", err)
		}

		return protoreflect.ValueOfMessage(m), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
