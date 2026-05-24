package patchproto

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Patcher interface {
	Patch(v proto.Message, delta *dpb.Delta, opts ...Option) error
}

// Option configures a Patch or Patched call.
type Option func(*PatchOption)

// WithTypes sets the type resolver used to decode MessageKind fields in
// Assigned operations. If not set, protoregistry.GlobalTypes is used.
func WithTypes(r protoregistry.MessageTypeResolver) Option {
	return func(o *PatchOption) {
		o.Types = r
	}
}

// WithHook registers a hook that is called each time a field is modified.
// The hook receives the path of PathEntries leading to the modified field.
func WithHook(h func([]dpb.PathEntry, *dpb.Entry)) Option {
	return func(o *PatchOption) {
		if o.cursor == nil {
			o.cursor = &dpb.Cursor{}
		}
		o.cursor.Hooks = append(o.cursor.Hooks, h)
	}
}

func Patch(v proto.Message, delta *dpb.Delta, opts ...Option) error {
	var o PatchOption
	for _, opt := range opts {
		opt(&o)
	}
	return o.Patch(v, delta)
}

func Patched[T proto.Message](v T, delta *dpb.Delta, opts ...Option) (T, error) {
	v = proto.CloneOf(v)
	if err := Patch(v, delta, opts...); err != nil {
		var z T
		return z, err
	}
	return v, nil
}

type PatchOption struct {
	// Types is used to resolve message types when decoding Assigned values for
	// MessageKind fields. If nil, protoregistry.GlobalTypes is used.
	Types protoregistry.MessageTypeResolver
	// cursor is a pointer so all value-receiver copies share the same path state.
	cursor *dpb.Cursor
}

func (o PatchOption) Patch(v proto.Message, delta *dpb.Delta) error {
	o_ := o
	c := &dpb.Cursor{}
	if o.cursor != nil {
		c.Hooks = o.cursor.Hooks
	}
	o_.cursor = c
	return o_.PatchField(v.ProtoReflect(), nil, delta)
}

// cursorEnter pushes a PathEntry and returns a function that pops it.
func (o PatchOption) cursorEnter(e dpb.PathEntry) func() {
	if o.cursor == nil {
		return func() {}
	}
	o.cursor.Push(e)
	return func() { o.cursor.Pop() }
}

// cursorNotify fires all hooks with the current cursor path and the entry being applied.
func (o PatchOption) cursorNotify(entry *dpb.Entry) {
	if o.cursor != nil {
		o.cursor.Notify(entry)
	}
}

func segmentToPathEntry(s any) dpb.PathEntry {
	switch s := s.(type) {
	case string:
		return dpb.PathEntry{Kind: dpb.PathEntryField, Key: s}
	case int:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: s}
	case uint:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(s)}
	default:
		return dpb.PathEntry{}
	}
}

func (o PatchOption) PatchField(v any, fd protoreflect.FieldDescriptor, delta *dpb.Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.patch(v, fd, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}

	return nil
}

func (o PatchOption) patch(v any, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	segments := slices.Collect(entry.Path().Seq())

	var s any
	if len(entry.GetTargets()) == 0 {
		segments, s = segments[:len(segments)-1], segments[len(segments)-1]
	}

	if o.cursor != nil {
		for _, seg := range segments {
			o.cursor.Push(segmentToPathEntry(seg))
		}
		defer func() {
			for range segments {
				o.cursor.Pop()
			}
		}()
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
			entry = entryWithTargets(entry, target.Fields(fd.Number()))

		case int:
			entry = entryWithTargets(entry, target.Fields(protoreflect.FieldNumber(s)))

		case uint:
			entry = entryWithTargets(entry, target.Fields(protoreflect.FieldNumber(s)))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type: %T", s)
		}
		return o.patchMessage(v, entry)

	case protoreflect.List:
		switch s := s.(type) {
		case int:
			entry = entryWithTargets(entry, target.Indices(s))

		case uint:
			entry = entryWithTargets(entry, target.Indices(int(s)))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type for list: %T", s)
		}
		return o.patchList(v, fd, entry)

	case protoreflect.Map:
		switch s := s.(type) {
		case string:
			entry = entryWithTargets(entry, target.StringKeys(s))

		case int:
			entry = entryWithTargets(entry, target.SignedKeys(s))

		case uint:
			entry = entryWithTargets(entry, target.UnsignedKeys(s))

		case nil:
		default:
			return fmt.Errorf("invalid target segment type: %T", s)
		}
		return o.patchMap(v, fd, entry)

	default:
		return fmt.Errorf("unsupported value type: %T", v)
	}
}

// entryWithTargets returns a shallow clone of entry with the given targets appended.
// The original entry is not modified.
func entryWithTargets(entry *dpb.Entry, targets target.Targets) *dpb.Entry {
	e := proto.Clone(entry).(*dpb.Entry)
	e.AppendTargets(targets)
	return e
}

// decodeValue decodes a field value from its wire-format bytes based on the field descriptor.
func (o PatchOption) decodeValue(b []byte, fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
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
		resolver := o.Types
		if resolver == nil {
			resolver = protoregistry.GlobalTypes
		}

		mt, err := resolver.FindMessageByName(fd.Message().FullName())
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
