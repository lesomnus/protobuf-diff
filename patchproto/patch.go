package patchproto

import (
	"bytes"
	"fmt"
	"math"

	"github.com/lesomnus/protobuf-diff/dpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Patcher interface {
	Patch(v proto.Message, delta *dpb.Delta, opts ...Option) error
}

// Option configures a Patch or Patched call.
type Option func(*PatchOption)

// WithTypes sets the type resolver used to decode message-kind values in
// Assign/Insert operations. If not set, protoregistry.GlobalTypes is used.
func WithTypes(r protoregistry.MessageTypeResolver) Option {
	return func(o *PatchOption) {
		o.Types = r
	}
}

// Frame holds the value and field descriptor at the cursor position.
type Frame struct {
	Descriptor protoreflect.FieldDescriptor
	Value      protoreflect.Value
}

func (f Frame) String() string {
	if !f.Value.IsValid() {
		return "<nil>"
	}
	if f.Descriptor != nil {
		switch f.Descriptor.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			return fmt.Sprint(f.Value.Message().Interface())
		}
	}
	return fmt.Sprint(f.Value.Interface())
}

// WithHook registers a hook called each time a field is modified.
func WithHook(h func([]dpb.PathEntry, dpb.Frame, dpb.Frame, *dpb.Entry)) Option {
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
	Types  protoregistry.MessageTypeResolver
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

func (o PatchOption) cursorEnter(e dpb.PathEntry) func() {
	if o.cursor == nil {
		return func() {}
	}
	o.cursor.Push(e)
	return func() { o.cursor.Pop() }
}

func (o PatchOption) cursorNotify(before, after dpb.Frame, entry *dpb.Entry) {
	if o.cursor != nil {
		o.cursor.Notify(before, after, entry)
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

func fieldSegmentToPathEntry(fs *dpb.FieldSegment) dpb.PathEntry {
	if fs.HasName() && fs.GetName() != "" {
		return dpb.PathEntry{Kind: dpb.PathEntryField, Key: fs.GetName()}
	}
	return dpb.PathEntry{Kind: dpb.PathEntryField, Index: int(fs.GetNumber())}
}

func (o PatchOption) patch(v any, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	pathSegs := entry.GetPath().GetSegments()

	if o.cursor != nil {
		for _, fs := range pathSegs {
			o.cursor.Push(fieldSegmentToPathEntry(fs))
		}
		defer func() {
			for range pathSegs {
				o.cursor.Pop()
			}
		}()
	}

	// A test only reads; every other kind mutates. Mutating navigation refuses
	// to descend into unset containers (which would panic on mutation).
	mutate := entry.WhichKind() != dpb.Entry_Test_case
	v, fd, err := navigate(v, fd, pathSegs, mutate)
	if err != nil {
		return fmt.Errorf("navigate path: %w", err)
	}

	targets := entry.GetTargets()

	switch c := v.(type) {
	case protoreflect.Message:
		return o.patchMessage(c, fd, targets, entry)
	case protoreflect.List:
		return o.patchList(c, fd, targets, entry)
	case protoreflect.Map:
		return o.patchMap(c, fd, targets, entry)
	default:
		return fmt.Errorf("unsupported container type: %T", v)
	}
}

// isNumericKind reports whether a field kind holds a numeric, enum, or bool
// value — the kinds that integer/unsigned Values can be converted to.
func isNumericKind(k protoreflect.Kind) bool {
	switch k {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind,
		protoreflect.BoolKind, protoreflect.EnumKind:
		return true
	default:
		return false
	}
}

// checkValueAssignable verifies that a Value's kind can be converted to the
// target field's kind before valueToProtoValue produces a protoreflect.Value.
// Without this guard a mismatched kind (e.g. a string Value for a message field)
// would build a wrong-typed value and panic at Set/Append time, or nil-deref
// fd.Message() for a message Value against a scalar field.
func checkValueAssignable(val *dpb.Value, fd protoreflect.FieldDescriptor) error {
	if fd == nil {
		return nil
	}
	k := fd.Kind()
	ok := true
	switch val.WhichKind() {
	case dpb.Value_M_case:
		ok = k == protoreflect.MessageKind || k == protoreflect.GroupKind
	case dpb.Value_F_case:
		ok = k == protoreflect.FloatKind || k == protoreflect.DoubleKind
	case dpb.Value_S_case:
		ok = k == protoreflect.StringKind
	case dpb.Value_B_case:
		ok = k == protoreflect.BoolKind
	case dpb.Value_X_case:
		ok = k == protoreflect.BytesKind
	case dpb.Value_I_case, dpb.Value_U_case:
		ok = isNumericKind(k)
	case dpb.Value_L_case:
		// Lists are handled at the container level; an L here is a mismatch.
		ok = false
	}
	if !ok {
		return fmt.Errorf("value kind %v is not assignable to %s (kind %v)", val.WhichKind(), fd.FullName(), k)
	}
	return nil
}

// valueToProtoValue converts a *dpb.Value to a protoreflect.Value using the field descriptor.
func valueToProtoValue(val *dpb.Value, fd protoreflect.FieldDescriptor, types protoregistry.MessageTypeResolver) (protoreflect.Value, error) {
	if val == nil {
		return protoreflect.Value{}, nil
	}
	if err := checkValueAssignable(val, fd); err != nil {
		return protoreflect.Value{}, err
	}
	switch val.WhichKind() {
	case dpb.Value_N_case:
		return protoreflect.Value{}, nil // invalid = clear/zero

	case dpb.Value_F_case:
		f := val.GetF()
		switch fd.Kind() {
		case protoreflect.FloatKind:
			return protoreflect.ValueOfFloat32(float32(f)), nil
		default:
			return protoreflect.ValueOfFloat64(f), nil
		}

	case dpb.Value_S_case:
		return protoreflect.ValueOfString(val.GetS()), nil

	case dpb.Value_B_case:
		return protoreflect.ValueOfBool(val.GetB()), nil

	case dpb.Value_I_case:
		i := val.GetI()
		switch fd.Kind() {
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			return protoreflect.ValueOfInt32(int32(i)), nil
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			return protoreflect.ValueOfInt64(i), nil
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			return protoreflect.ValueOfUint32(uint32(i)), nil
		case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			return protoreflect.ValueOfUint64(uint64(i)), nil
		case protoreflect.FloatKind:
			return protoreflect.ValueOfFloat32(float32(i)), nil
		case protoreflect.DoubleKind:
			return protoreflect.ValueOfFloat64(float64(i)), nil
		case protoreflect.BoolKind:
			return protoreflect.ValueOfBool(i != 0), nil
		case protoreflect.EnumKind:
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(i)), nil
		default:
			return protoreflect.ValueOfInt64(i), nil
		}

	case dpb.Value_U_case:
		u := val.GetU()
		switch fd.Kind() {
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			return protoreflect.ValueOfUint32(uint32(u)), nil
		case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			return protoreflect.ValueOfUint64(u), nil
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			return protoreflect.ValueOfInt32(int32(u)), nil
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			return protoreflect.ValueOfInt64(int64(u)), nil
		case protoreflect.FloatKind:
			return protoreflect.ValueOfFloat32(float32(u)), nil
		case protoreflect.DoubleKind:
			return protoreflect.ValueOfFloat64(float64(u)), nil
		case protoreflect.BoolKind:
			return protoreflect.ValueOfBool(u != 0), nil
		case protoreflect.EnumKind:
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(u)), nil
		default:
			return protoreflect.ValueOfUint64(u), nil
		}

	case dpb.Value_X_case:
		return protoreflect.ValueOfBytes(val.GetX()), nil

	case dpb.Value_M_case:
		if types == nil {
			types = protoregistry.GlobalTypes
		}
		mt, err := types.FindMessageByName(fd.Message().FullName())
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("find message type %q: %w", fd.Message().FullName(), err)
		}
		m := mt.New()
		if err := applyStructToMessage(val.GetM(), m, types); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(m), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported value kind: %v", val.WhichKind())
	}
}

// applyStructToMessage populates a message from a Struct. Fields present in the
// struct are set (or cleared when the value is null/unset); fields not mentioned
// are left untouched.
func applyStructToMessage(s *dpb.Struct, m protoreflect.Message, types protoregistry.MessageTypeResolver) error {
	if s == nil {
		return nil
	}
	fields := m.Descriptor().Fields()
	for _, kv := range s.GetFields() {
		fd := findFieldByFieldSegment(fields, kv.GetKey())
		if fd == nil {
			continue
		}
		if err := setMessageField(m, fd, kv.GetValue(), types); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}
	}
	return nil
}

// setMessageField sets a single field of m from a dpb.Value, dispatching on
// whether the field is a list, a map, or a scalar/message. List and map fields
// are replaced wholesale (any existing contents are discarded).
func setMessageField(m protoreflect.Message, fd protoreflect.FieldDescriptor, val *dpb.Value, types protoregistry.MessageTypeResolver) error {
	switch {
	case fd.IsList():
		return setListField(m, fd, val, types)
	case fd.IsMap():
		return setMapField(m, fd, val, types)
	default:
		v, err := valueToProtoValue(val, fd, types)
		if err != nil {
			return err
		}
		if !v.IsValid() {
			m.Clear(fd)
		} else {
			m.Set(fd, v)
		}
		return nil
	}
}

// isClearValue reports whether a Value means "clear the field" (nil, unset, or
// the explicit null value).
func isClearValue(val *dpb.Value) bool {
	if val == nil {
		return true
	}
	switch val.WhichKind() {
	case dpb.Value_Kind_not_set_case, dpb.Value_N_case:
		return true
	default:
		return false
	}
}

// setListField replaces a repeated field with the elements of a ListValue.
func setListField(m protoreflect.Message, fd protoreflect.FieldDescriptor, val *dpb.Value, types protoregistry.MessageTypeResolver) error {
	if isClearValue(val) {
		m.Clear(fd)
		return nil
	}
	if val.WhichKind() != dpb.Value_L_case {
		return fmt.Errorf("expected list value for repeated field, got %v", val.WhichKind())
	}
	lst := m.NewField(fd).List()
	if err := appendDpbValues(lst, val.GetL().GetValues(), fd, types); err != nil {
		return err
	}
	m.Set(fd, protoreflect.ValueOfList(lst))
	return nil
}

// setMapField replaces a map field with the entries of a Struct whose KeyValue
// keys are the map keys.
func setMapField(m protoreflect.Message, fd protoreflect.FieldDescriptor, val *dpb.Value, types protoregistry.MessageTypeResolver) error {
	if isClearValue(val) {
		m.Clear(fd)
		return nil
	}
	if val.WhichKind() != dpb.Value_M_case {
		return fmt.Errorf("expected struct value for map field, got %v", val.WhichKind())
	}
	mp := m.NewField(fd).Map()
	if err := setMapEntries(mp, fd.MapKey(), fd.MapValue(), val.GetM().GetFields(), types, false); err != nil {
		return err
	}
	m.Set(fd, protoreflect.ValueOfMap(mp))
	return nil
}

// appendDpbValues converts each dpb.Value against the (element) descriptor fd
// and appends it to list, dropping null/invalid elements. Shared by whole-list
// replace in a message field (setListField) and at a list root (patchListRoot).
func appendDpbValues(list protoreflect.List, vals []*dpb.Value, fd protoreflect.FieldDescriptor, types protoregistry.MessageTypeResolver) error {
	for i, ev := range vals {
		pv, err := valueToProtoValue(ev, fd, types)
		if err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
		if !pv.IsValid() {
			continue // null element — dropped
		}
		list.Append(pv)
	}
	return nil
}

// setMapEntries sets each Struct KeyValue into mp as a map entry, keyed by the
// FieldSegment. When onlyAbsent is true, keys already present are left untouched
// (merge). Null/invalid values are dropped. Shared by whole-map replace in a
// message field (setMapField) and at a map root (patchMapRoot).
func setMapEntries(mp protoreflect.Map, kd, vd protoreflect.FieldDescriptor, fields []*dpb.KeyValue, types protoregistry.MessageTypeResolver, onlyAbsent bool) error {
	for _, kv := range fields {
		mk, err := fieldSegmentToMapKey(kv.GetKey(), kd.Kind())
		if err != nil {
			return err
		}
		if onlyAbsent && mp.Has(mk) {
			continue
		}
		pv, err := valueToProtoValue(kv.GetValue(), vd, types)
		if err != nil {
			return fmt.Errorf("[%v]: %w", mk, err)
		}
		if !pv.IsValid() {
			continue // null value — dropped
		}
		mp.Set(mk, pv)
	}
	return nil
}

// protoValueEqual compares two protoreflect.Values for equality by field kind.
func protoValueEqual(a, b protoreflect.Value, kind protoreflect.Kind) bool {
	if !a.IsValid() && !b.IsValid() {
		return true
	}
	if !a.IsValid() || !b.IsValid() {
		return false
	}
	switch kind {
	case protoreflect.BoolKind:
		return a.Bool() == b.Bool()
	case protoreflect.EnumKind:
		return a.Enum() == b.Enum()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return a.Int() == b.Int()
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return a.Uint() == b.Uint()
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		af, bf := a.Float(), b.Float()
		return af == bf || (math.IsNaN(af) && math.IsNaN(bf))
	case protoreflect.StringKind:
		return a.String() == b.String()
	case protoreflect.BytesKind:
		return bytes.Equal(a.Bytes(), b.Bytes())
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return proto.Equal(a.Message().Interface(), b.Message().Interface())
	default:
		return false
	}
}

// findFieldByFieldSegment finds a FieldDescriptor by FieldSegment (name, name_alt, or number).
func findFieldByFieldSegment(fields protoreflect.FieldDescriptors, fs *dpb.FieldSegment) protoreflect.FieldDescriptor {
	if fs == nil {
		return nil
	}
	if fs.HasName() && fs.GetName() != "" {
		if fd := fields.ByName(protoreflect.Name(fs.GetName())); fd != nil {
			return fd
		}
	}
	if fs.HasNameAlt() && fs.GetNameAlt() != "" {
		if fd := fields.ByJSONName(fs.GetNameAlt()); fd != nil {
			return fd
		}
	}
	if fs.HasNumber() {
		if fd := fields.ByNumber(protoreflect.FieldNumber(fs.GetNumber())); fd != nil {
			return fd
		}
	}
	return nil
}
