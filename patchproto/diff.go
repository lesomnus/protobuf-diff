package patchproto

import (
	"bytes"
	"fmt"
	"math"

	"github.com/lesomnus/protobuf-diff/dpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Differ interface {
	Diff(from, to proto.Message) (*dpb.Delta, error)
}

type DiffOption struct{}

func Diff[T proto.Message](from, to T) (*dpb.Delta, error) {
	return DiffOption{}.Diff(from, to)
}

func (o DiffOption) Diff(from, to proto.Message) (*dpb.Delta, error) {
	entries, err := o.diffMessage(from.ProtoReflect(), to.ProtoReflect())
	if err != nil {
		return nil, err
	}
	return dpb.NewDelta(entries...), nil
}

func (o DiffOption) diffMessage(from, to protoreflect.Message) ([]*dpb.Entry, error) {
	var entries []*dpb.Entry
	fields := from.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		es, err := o.diffField(from, to, fd)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", fd.FullName(), err)
		}
		entries = append(entries, es...)
	}
	return entries, nil
}

func segForField(num protoreflect.FieldNumber) *dpb.Segment {
	return dpb.SegField(dpb.FieldNum(int64(num)))
}

func segForIndex(i int) *dpb.Segment {
	return dpb.SegIndex(int64(i))
}

func segForStringKey(k string) *dpb.Segment {
	return dpb.SegName(k)
}

func (o DiffOption) diffField(from, to protoreflect.Message, fd protoreflect.FieldDescriptor) ([]*dpb.Entry, error) {
	lhs_has := from.Has(fd)
	rhs_has := to.Has(fd)
	if !lhs_has && !rhs_has {
		return nil, nil
	}

	if lhs_has && !rhs_has {
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
		e.SetRemove(true)
		return []*dpb.Entry{e}, nil
	}

	if fd.IsMap() {
		nested, err := o.diffMap(from.Get(fd).Map(), to.Get(fd).Map(), fd)
		if err != nil {
			return nil, err
		}
		if len(nested) == 0 {
			return nil, nil
		}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
		e.SetNest(dpb.NewDelta(nested...))
		return []*dpb.Entry{e}, nil
	}

	if fd.IsList() {
		nested, err := o.diffList(from.Get(fd).List(), to.Get(fd).List(), fd)
		if err != nil {
			return nil, err
		}
		if len(nested) == 0 {
			return nil, nil
		}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
		e.SetNest(dpb.NewDelta(nested...))
		return []*dpb.Entry{e}, nil
	}

	kind := fd.Kind()
	if kind == protoreflect.MessageKind || kind == protoreflect.GroupKind {
		if !lhs_has {
			v, err := toValue(to.Get(fd), fd)
			if err != nil {
				return nil, err
			}
			e := &dpb.Entry{}
			e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
			e.SetAssign(v)
			return []*dpb.Entry{e}, nil
		}
		nested, err := o.diffMessage(from.Get(fd).Message(), to.Get(fd).Message())
		if err != nil {
			return nil, err
		}
		if len(nested) == 0 {
			return nil, nil
		}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
		e.SetNest(dpb.NewDelta(nested...))
		return []*dpb.Entry{e}, nil
	}

	lhs_v := from.Get(fd)
	rhs_v := to.Get(fd)
	if lhs_has && scalarValuesEqual(lhs_v, rhs_v, kind) {
		return nil, nil
	}

	v, err := toValue(rhs_v, fd)
	if err != nil {
		return nil, err
	}
	e := &dpb.Entry{}
	e.SetTargets([]*dpb.Segment{segForField(fd.Number())})
	e.SetAssign(v)
	return []*dpb.Entry{e}, nil
}

func (o DiffOption) diffList(from, to protoreflect.List, fd protoreflect.FieldDescriptor) ([]*dpb.Entry, error) {
	var entries []*dpb.Entry
	lhs_l := from.Len()
	rhs_l := to.Len()

	if lhs_l > rhs_l {
		segs := make([]*dpb.Segment, lhs_l-rhs_l)
		for i := range segs {
			segs[i] = segForIndex(rhs_l + i)
		}
		e := &dpb.Entry{}
		e.SetTargets(segs)
		e.SetRemove(true)
		entries = append(entries, e)
	}

	kind := fd.Kind()
	isMsg := kind == protoreflect.MessageKind || kind == protoreflect.GroupKind
	for i := range min(lhs_l, rhs_l) {
		lhs_v := from.Get(i)
		rhs_v := to.Get(i)

		if isMsg {
			nested, err := o.diffMessage(lhs_v.Message(), rhs_v.Message())
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			if len(nested) > 0 {
				e := &dpb.Entry{}
				e.SetTargets([]*dpb.Segment{segForIndex(i)})
				e.SetNest(dpb.NewDelta(nested...))
				entries = append(entries, e)
			}
			continue
		}

		if scalarValuesEqual(lhs_v, rhs_v, kind) {
			continue
		}
		v, err := toValue(rhs_v, fd)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForIndex(i)})
		e.SetAssign(v)
		entries = append(entries, e)
	}

	for i := lhs_l; i < rhs_l; i++ {
		v, err := toValue(to.Get(i), fd)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{segForIndex(-1)}) // append
		e.SetInsert(v)
		entries = append(entries, e)
	}

	return entries, nil
}

func (o DiffOption) diffMap(from, to protoreflect.Map, fd protoreflect.FieldDescriptor) ([]*dpb.Entry, error) {
	kd := fd.MapKey()
	vd := fd.MapValue()
	var entries []*dpb.Entry

	var keys_deleted []protoreflect.MapKey
	from.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
		if !to.Has(k) {
			keys_deleted = append(keys_deleted, k)
		}
		return true
	})
	if len(keys_deleted) > 0 {
		segs, err := mapKeysToSegments(keys_deleted, kd.Kind())
		if err != nil {
			return nil, err
		}
		e := &dpb.Entry{}
		e.SetTargets(segs)
		e.SetRemove(true)
		entries = append(entries, e)
	}

	type MapDiff struct {
		key   protoreflect.MapKey
		lhs_v protoreflect.Value
		rhs_v protoreflect.Value
		has   bool
	}
	var diffs []MapDiff
	to.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		diffs = append(diffs, MapDiff{k, from.Get(k), v, from.Has(k)})
		return true
	})

	if vd.Kind() == protoreflect.MessageKind || vd.Kind() == protoreflect.GroupKind {
		for _, d := range diffs {
			seg, err := mapKeyToSegment(d.key, kd.Kind())
			if err != nil {
				return nil, err
			}
			if d.has {
				nested, err := o.diffMessage(d.lhs_v.Message(), d.rhs_v.Message())
				if err != nil {
					return nil, fmt.Errorf("key %v: %w", d.key, err)
				}
				if len(nested) == 0 {
					continue
				}
				e := &dpb.Entry{}
				e.SetTargets([]*dpb.Segment{seg})
				e.SetNest(dpb.NewDelta(nested...))
				entries = append(entries, e)
			} else {
				v, err := toValue(d.rhs_v, vd)
				if err != nil {
					return nil, fmt.Errorf("key %v: %w", d.key, err)
				}
				e := &dpb.Entry{}
				e.SetTargets([]*dpb.Segment{seg})
				e.SetAssign(v)
				entries = append(entries, e)
			}
		}
	} else {
		for _, d := range diffs {
			if d.has && scalarValuesEqual(d.lhs_v, d.rhs_v, vd.Kind()) {
				continue
			}
			seg, err := mapKeyToSegment(d.key, kd.Kind())
			if err != nil {
				return nil, fmt.Errorf("key %v: %w", d.key, err)
			}
			v, err := toValue(d.rhs_v, vd)
			if err != nil {
				return nil, fmt.Errorf("key %v: %w", d.key, err)
			}
			e := &dpb.Entry{}
			e.SetTargets([]*dpb.Segment{seg})
			e.SetAssign(v)
			entries = append(entries, e)
		}
	}

	return entries, nil
}

func scalarValuesEqual(a, b protoreflect.Value, kind protoreflect.Kind) bool {
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
	default:
		return false
	}
}

// toValue converts a protoreflect.Value to a *dpb.Value.
func toValue(v protoreflect.Value, fd protoreflect.FieldDescriptor) (*dpb.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return dpb.ValB(v.Bool()), nil
	case protoreflect.EnumKind:
		return dpb.ValU(uint64(v.Enum())), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return dpb.ValI(v.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return dpb.ValU(v.Uint()), nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return dpb.ValF(v.Float()), nil
	case protoreflect.StringKind:
		return dpb.ValS(v.String()), nil
	case protoreflect.BytesKind:
		return dpb.ValX(v.Bytes()), nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		s, err := messageToStruct(v.Message())
		if err != nil {
			return nil, err
		}
		val := &dpb.Value{}
		val.SetM(s)
		return val, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %v", fd.Kind())
	}
}

// messageToStruct converts a protoreflect.Message to a *dpb.Struct.
func messageToStruct(m protoreflect.Message) (*dpb.Struct, error) {
	var fields []*dpb.KeyValue
	var retErr error
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		val, err := toValue(v, fd)
		if err != nil {
			retErr = fmt.Errorf("field %s: %w", fd.Name(), err)
			return false
		}
		kv := &dpb.KeyValue{}
		kv.SetKey(dpb.FieldNum(int64(fd.Number())))
		kv.SetValue(val)
		fields = append(fields, kv)
		return true
	})
	if retErr != nil {
		return nil, retErr
	}
	s := &dpb.Struct{}
	s.SetFields(fields)
	return s, nil
}

// mapKeyToSegment converts a MapKey to a *dpb.Segment.
func mapKeyToSegment(k protoreflect.MapKey, kind protoreflect.Kind) (*dpb.Segment, error) {
	switch kind {
	case protoreflect.StringKind:
		return segForStringKey(k.String()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return dpb.SegIndex(k.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return dpb.SegIndex(int64(k.Uint())), nil
	case protoreflect.BoolKind:
		if k.Bool() {
			return dpb.SegIndex(1), nil
		}
		return dpb.SegIndex(0), nil
	default:
		return nil, fmt.Errorf("unsupported map key kind: %v", kind)
	}
}

// mapKeysToSegments converts multiple MapKeys to []*dpb.Segment.
func mapKeysToSegments(keys []protoreflect.MapKey, kind protoreflect.Kind) ([]*dpb.Segment, error) {
	segs := make([]*dpb.Segment, len(keys))
	for i, k := range keys {
		s, err := mapKeyToSegment(k, kind)
		if err != nil {
			return nil, err
		}
		segs[i] = s
	}
	return segs, nil
}
