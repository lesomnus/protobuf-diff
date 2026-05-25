package dpb

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ValM creates a Value from a proto message by encoding all set scalar/message fields into a Struct.
func ValM(m proto.Message) *Value {
	s := protoMsgToStruct(m.ProtoReflect())
	v := &Value{}
	v.SetM(s)
	return v
}

func protoMsgToStruct(m protoreflect.Message) *Struct {
	var fields []*KeyValue
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsList() || fd.IsMap() {
			return true
		}
		val := protoValToVal(v, fd)
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
