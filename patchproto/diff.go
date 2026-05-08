package patchproto

import (
	"bytes"
	"fmt"
	"math"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/encoding/protowire"
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
		if es == nil {
			continue
		}
		entries = append(entries, es...)
	}
	return entries, nil
}

func (o DiffOption) diffField(from, to protoreflect.Message, fd protoreflect.FieldDescriptor) ([]*dpb.Entry, error) {
	lhs_has := from.Has(fd)
	rhs_has := to.Has(fd)
	if !lhs_has && !rhs_has {
		return nil, nil
	}

	if lhs_has && !rhs_has {
		e := &dpb.Entry{}
		e.AppendTargets(target.Fields(fd.Number()))
		e.SetDeleted(true)
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
		e.AppendTargets(target.Fields(fd.Number()))
		e.SetNested(dpb.NewDelta(nested...))
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
		e.AppendTargets(target.Fields(fd.Number()))
		e.SetNested(dpb.NewDelta(nested...))
		return []*dpb.Entry{e}, nil
	}

	kind := fd.Kind()
	if kind == protoreflect.MessageKind || kind == protoreflect.GroupKind {
		if !lhs_has {
			b, err := encodeValue(to.Get(fd), fd)
			if err != nil {
				return nil, err
			}
			e := &dpb.Entry{}
			e.AppendTargets(target.Fields(fd.Number()))
			e.SetAssigned(b)
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
		e.AppendTargets(target.Fields(fd.Number()))
		e.SetNested(dpb.NewDelta(nested...))
		return []*dpb.Entry{e}, nil
	}

	lhs_v := from.Get(fd)
	rhs_v := to.Get(fd)
	if lhs_has && scalarValuesEqual(lhs_v, rhs_v, kind) {
		return nil, nil
	}

	b, err := encodeValue(rhs_v, fd)
	if err != nil {
		return nil, err
	}
	e := &dpb.Entry{}
	e.AppendTargets(target.Fields(fd.Number()))
	e.SetAssigned(b)
	return []*dpb.Entry{e}, nil
}

func (o DiffOption) diffList(from, to protoreflect.List, fd protoreflect.FieldDescriptor) ([]*dpb.Entry, error) {
	var entries []*dpb.Entry
	lhs_l := from.Len()
	rhs_l := to.Len()

	if lhs_l > rhs_l {
		indices := make([]int, lhs_l-rhs_l)
		for i := range indices {
			indices[i] = rhs_l + i
		}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(indices...))
		e.SetDeleted(true)
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
				e.AppendTargets(target.Indices(i))
				e.SetNested(dpb.NewDelta(nested...))
				entries = append(entries, e)
			}
			continue
		}

		if scalarValuesEqual(lhs_v, rhs_v, kind) {
			continue
		}
		b, err := encodeValue(rhs_v, fd)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(i))
		e.SetAssigned(b)
		entries = append(entries, e)
	}

	for i := lhs_l; i < rhs_l; i++ {
		b, err := encodeValue(to.Get(i), fd)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		e := &dpb.Entry{}
		e.SetNoUpdate(true)
		e.AppendTargets(target.Indices(-1))
		e.SetAssigned(b)
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
		bs, err := encodeMapKeyTargets(keys_deleted, kd.Kind())
		if err != nil {
			return nil, err
		}
		e := &dpb.Entry{}
		e.SetTargets(bs)
		e.SetDeleted(true)
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
			if d.has {
				nested, err := o.diffMessage(d.lhs_v.Message(), d.rhs_v.Message())
				if err != nil {
					return nil, fmt.Errorf("key %v: %w", d.key, err)
				}
				if len(nested) == 0 {
					continue
				}
				bs, err := encodeMapKeyTargets([]protoreflect.MapKey{d.key}, kd.Kind())
				if err != nil {
					return nil, err
				}
				e := &dpb.Entry{}
				e.SetTargets(bs)
				e.SetNested(dpb.NewDelta(nested...))
				entries = append(entries, e)
			} else {
				b, err := encodeValue(d.rhs_v, vd)
				if err != nil {
					return nil, fmt.Errorf("key %v: %w", d.key, err)
				}
				bs, err := encodeMapKeyTargets([]protoreflect.MapKey{d.key}, kd.Kind())
				if err != nil {
					return nil, err
				}
				e := &dpb.Entry{}
				e.SetTargets(bs)
				e.SetAssigned(b)
				entries = append(entries, e)
			}
		}
	} else {
		for _, d := range diffs {
			if d.has && scalarValuesEqual(d.lhs_v, d.rhs_v, vd.Kind()) {
				continue
			}
			b, err := encodeValue(d.rhs_v, vd)
			if err != nil {
				return nil, fmt.Errorf("key %v: %w", d.key, err)
			}
			bs, err := encodeMapKeyTargets([]protoreflect.MapKey{d.key}, kd.Kind())
			if err != nil {
				return nil, err
			}
			e := &dpb.Entry{}
			e.SetTargets(bs)
			e.SetAssigned(b)
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

// encodeValue encodes a field value into wire-format bytes for use in assigned entries.
// It is the inverse of decodeValue in patch.go.
func encodeValue(v protoreflect.Value, fd protoreflect.FieldDescriptor) ([]byte, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return dpb.Bool(v.Bool()), nil
	case protoreflect.EnumKind:
		return dpb.Enum(v.Enum()), nil
	case protoreflect.Int32Kind:
		return dpb.Int(v.Int()), nil
	case protoreflect.Sint32Kind:
		return dpb.Signed(v.Int()), nil
	case protoreflect.Uint32Kind:
		return dpb.Int(v.Uint()), nil
	case protoreflect.Int64Kind:
		return dpb.Int(v.Int()), nil
	case protoreflect.Sint64Kind:
		return dpb.Signed(v.Int()), nil
	case protoreflect.Uint64Kind:
		return dpb.Int(v.Uint()), nil
	case protoreflect.Sfixed32Kind:
		return protowire.AppendFixed32(nil, uint32(v.Int())), nil
	case protoreflect.Fixed32Kind:
		return protowire.AppendFixed32(nil, uint32(v.Uint())), nil
	case protoreflect.FloatKind:
		return dpb.Float(float32(v.Float())), nil
	case protoreflect.Sfixed64Kind:
		return protowire.AppendFixed64(nil, uint64(v.Int())), nil
	case protoreflect.Fixed64Kind:
		return protowire.AppendFixed64(nil, v.Uint()), nil
	case protoreflect.DoubleKind:
		return dpb.Double(v.Float()), nil
	case protoreflect.StringKind:
		return dpb.String(v.String()), nil
	case protoreflect.BytesKind:
		return dpb.Bytes(v.Bytes()), nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		b, err := proto.Marshal(v.Message().Interface())
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %v", fd.Kind())
	}
}

// encodeMapKeyTargets encodes map keys into the targets bytes format,
// matching the decoding logic in target.DecodeKeys.
func encodeMapKeyTargets(keys []protoreflect.MapKey, kind protoreflect.Kind) ([]byte, error) {
	var bs []byte
	for _, k := range keys {
		switch kind {
		case protoreflect.BoolKind:
			v := uint64(0)
			if k.Bool() {
				v = 1
			}
			bs = protowire.AppendVarint(bs, v)
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			bs = protowire.AppendVarint(bs, uint64(k.Int()))
		case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
			bs = protowire.AppendVarint(bs, protowire.EncodeZigZag(k.Int()))
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
			bs = protowire.AppendVarint(bs, k.Uint())
		case protoreflect.Sfixed32Kind:
			bs = protowire.AppendFixed32(bs, uint32(k.Int()))
		case protoreflect.Fixed32Kind:
			bs = protowire.AppendFixed32(bs, uint32(k.Uint()))
		case protoreflect.Sfixed64Kind:
			bs = protowire.AppendFixed64(bs, uint64(k.Int()))
		case protoreflect.Fixed64Kind:
			bs = protowire.AppendFixed64(bs, k.Uint())
		case protoreflect.StringKind:
			bs = protowire.AppendString(bs, k.String())
		default:
			return nil, fmt.Errorf("unsupported map key kind: %v", kind)
		}
	}
	return bs, nil
}
