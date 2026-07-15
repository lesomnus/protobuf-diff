package dpb

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ValM creates a Value from a proto message by encoding all set fields
// (scalar, message, repeated, and map) into a Struct.
func ValM(m proto.Message) *Value {
	s := protoMsgToStruct(m.ProtoReflect())
	v := &Value{}
	v.SetM(s)
	return v
}

// ReplaceWith builds an Entry with no targets whose assign replaces the
// container it is applied to — the root message, or the message reached by the
// entry's path — with the contents of m. Set a path on the returned entry to
// target a nested message.
//
//	e := dpb.ReplaceWith(newMsg)          // replace the whole root message
//	e.SetPath(dpb.PathOf(dpb.Field("m"))) // replace field "m" wholesale
func ReplaceWith(m proto.Message) *Entry {
	e := &Entry{}
	e.SetAssign(ValM(m))
	return e
}

func protoMsgToStruct(m protoreflect.Message) *Struct {
	var fields []*KeyValue
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		var val *Value
		switch {
		case fd.IsList():
			val = protoListToValue(v.List(), fd)
		case fd.IsMap():
			val = protoMapToValue(v.Map(), fd)
		default:
			val = protoValToVal(v, fd)
		}
		if val == nil {
			return true
		}
		kv := &KeyValue{}
		kv.SetKey(FieldNum(int64(fd.Number())))
		kv.SetValue(val)
		fields = append(fields, kv)
		return true
	})
	s := &Struct{}
	s.SetFields(fields)
	return s
}

// protoListToValue encodes a repeated field as a Value wrapping a ListValue.
// Each element is encoded with the list's field descriptor (whose Kind is the
// element kind).
func protoListToValue(l protoreflect.List, fd protoreflect.FieldDescriptor) *Value {
	vals := make([]*Value, 0, l.Len())
	for i := range l.Len() {
		ev := protoValToVal(l.Get(i), fd)
		if ev == nil {
			ev = ValNull()
		}
		vals = append(vals, ev)
	}
	lv := &ListValue{}
	lv.SetValues(vals)
	v := &Value{}
	v.SetL(lv)
	return v
}

// protoMapToValue encodes a map field as a Value wrapping a Struct whose
// KeyValue keys are the map keys and values are the map values.
func protoMapToValue(mp protoreflect.Map, fd protoreflect.FieldDescriptor) *Value {
	kd := fd.MapKey()
	vd := fd.MapValue()
	var kvs []*KeyValue
	mp.Range(func(mk protoreflect.MapKey, v protoreflect.Value) bool {
		val := protoValToVal(v, vd)
		if val == nil {
			return true
		}
		kv := &KeyValue{}
		kv.SetKey(mapKeyToFieldSegment(mk, kd))
		kv.SetValue(val)
		kvs = append(kvs, kv)
		return true
	})
	s := &Struct{}
	s.SetFields(kvs)
	v := &Value{}
	v.SetM(s)
	return v
}

// mapKeyToFieldSegment encodes a map key as a FieldSegment: string keys use the
// name, all other (integer/bool) keys use the number.
func mapKeyToFieldSegment(mk protoreflect.MapKey, kd protoreflect.FieldDescriptor) *FieldSegment {
	switch kd.Kind() {
	case protoreflect.StringKind:
		return Field(mk.Value().String())
	case protoreflect.BoolKind:
		if mk.Value().Bool() {
			return FieldNum(1)
		}
		return FieldNum(0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return FieldNum(int64(mk.Value().Uint()))
	default:
		return FieldNum(mk.Value().Int())
	}
}

func protoValToVal(v protoreflect.Value, fd protoreflect.FieldDescriptor) *Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return ValB(v.Bool())
	case protoreflect.EnumKind:
		return ValU(uint64(v.Enum()))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return ValI(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return ValU(v.Uint())
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return ValF(v.Float())
	case protoreflect.StringKind:
		return ValS(v.String())
	case protoreflect.BytesKind:
		return ValX(v.Bytes())
	case protoreflect.MessageKind, protoreflect.GroupKind:
		val := &Value{}
		val.SetM(protoMsgToStruct(v.Message()))
		return val
	default:
		return nil
	}
}

// SegName creates a Segment targeting by string name (field name or string map key).
func SegName(name string) *Segment {
	s := &Segment{}
	s.SetName(name)
	return s
}

// SegIndex creates a Segment targeting by signed integer index.
func SegIndex(index int64) *Segment {
	s := &Segment{}
	s.SetIndex(index)
	return s
}

// SegField creates a Segment targeting by a FieldSegment.
func SegField(fs *FieldSegment) *Segment {
	s := &Segment{}
	s.SetField(fs)
	return s
}

// Field creates a FieldSegment identified by name.
func Field(name string) *FieldSegment {
	fs := &FieldSegment{}
	fs.SetName(name)
	return fs
}

// FieldNum creates a FieldSegment identified by field number (or list index).
func FieldNum(number int64) *FieldSegment {
	fs := &FieldSegment{}
	fs.SetNumber(number)
	return fs
}

// PathOf creates a Path from a sequence of FieldSegments.
func PathOf(segs ...*FieldSegment) *Path {
	return Path_builder{Segments: segs}.Build()
}

// ValNull creates a null Value.
func ValNull() *Value {
	v := &Value{}
	v.SetN(NullValue_NULL_VALUE)
	return v
}

// ValF creates a float64 Value.
func ValF(f float64) *Value {
	v := &Value{}
	v.SetF(f)
	return v
}

// ValS creates a string Value.
func ValS(s string) *Value {
	v := &Value{}
	v.SetS(s)
	return v
}

// ValB creates a bool Value.
func ValB(b bool) *Value {
	v := &Value{}
	v.SetB(b)
	return v
}

// ValI creates a signed int64 Value.
func ValI(i int64) *Value {
	v := &Value{}
	v.SetI(i)
	return v
}

// ValU creates an unsigned uint64 Value.
func ValU(u uint64) *Value {
	v := &Value{}
	v.SetU(u)
	return v
}

// ValX creates a bytes Value.
func ValX(x []byte) *Value {
	v := &Value{}
	v.SetX(x)
	return v
}

// ValL creates a Value wrapping a list of values, e.g. for replacing a repeated
// field wholesale at its root.
func ValL(vs ...*Value) *Value {
	lv := &ListValue{}
	lv.SetValues(vs)
	v := &Value{}
	v.SetL(lv)
	return v
}
