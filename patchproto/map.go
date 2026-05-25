package patchproto

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func mapKeyToPathEntry(k protoreflect.MapKey) dpb.PathEntry {
	switch v := k.Interface().(type) {
	case string:
		return dpb.PathEntry{Kind: dpb.PathEntryField, Key: v}
	case int32:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(v)}
	case int64:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(v)}
	case uint32:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(v)}
	case uint64:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(v)}
	default:
		return dpb.PathEntry{}
	}
}

func (o PatchOption) patchMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, targets []*dpb.Segment, entry *dpb.Entry) error {
	if len(targets) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.PatchField(c, fd, entry.GetNest())
		}
		return nil
	}

	kd := fd.MapKey()
	vd := fd.MapValue()

	keys, err := mapKeysFromSegments(targets, kd.Kind())
	if err != nil {
		return fmt.Errorf("decode keys: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	notify_leaf := true
	op := func(k protoreflect.MapKey) error { return nil }
	after := func() error { return nil }
	after_notify := func() {}

	switch entry.WhichKind() {
	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			op = func(k protoreflect.MapKey) error {
				c.Clear(k)
				return nil
			}
		}

	case dpb.Entry_Test_case:
		notify_leaf = false
		val := entry.GetTest()
		op = func(k protoreflect.MapKey) error {
			current := c.Get(k)
			expected, err := valueToProtoValue(val, vd, o.Types)
			if err != nil {
				return fmt.Errorf("test: decode: %w", err)
			}
			if !protoValueEqual(current, expected, vd.Kind()) {
				return fmt.Errorf("test failed at key %v", k)
			}
			return nil
		}

	case dpb.Entry_Insert_case:
		val := entry.GetInsert()
		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if c.Has(k) {
				return nil // already present
			}
			if !v.IsValid() {
				var err error
				v, err = valueToProtoValue(val, vd, o.Types)
				if err != nil {
					return fmt.Errorf("insert: decode: %w", err)
				}
			}
			c.Set(k, v)
			return nil
		}

	case dpb.Entry_Assign_case:
		val := entry.GetAssign()
		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if !v.IsValid() {
				var err error
				v, err = valueToProtoValue(val, vd, o.Types)
				if err != nil {
					return fmt.Errorf("assign: decode: %w", err)
				}
			}
			c.Set(k, v)
			return nil
		}

	case dpb.Entry_Move_case:
		src := entry.GetMove()
		srcKey, err := fieldSegmentToMapKey(src, kd.Kind())
		if err != nil {
			return fmt.Errorf("move: %w", err)
		}
		src_before := Frame{Descriptor: vd, Value: c.Get(srcKey)}
		if !c.Has(srcKey) {
			op = func(k protoreflect.MapKey) error {
				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !v.IsValid() {
					v = c.Get(srcKey)
				}
				c.Set(k, v)
				return nil
			}
		}
		after = func() error {
			c.Clear(srcKey)
			return nil
		}
		after_notify = func() {
			leave := o.cursorEnter(mapKeyToPathEntry(srcKey))
			o.cursorNotify(src_before, Frame{Descriptor: vd}, entry)
			leave()
		}

	case dpb.Entry_Copy_case:
		src := entry.GetCopy()
		srcKey, err := fieldSegmentToMapKey(src, kd.Kind())
		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}
		if !c.Has(srcKey) {
			op = func(k protoreflect.MapKey) error {
				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !v.IsValid() {
					v = c.Get(srcKey)
				}
				c.Set(k, v)
				return nil
			}
		}

	case dpb.Entry_Nest_case:
		notify_leaf = false
		delta := entry.GetNest()
		kind := vd.Kind()
		if !(kind == protoreflect.MessageKind || kind == protoreflect.GroupKind) {
			return fmt.Errorf("nest for maps requires message value kind, got %q", kind)
		}
		op = func(k protoreflect.MapKey) error {
			if !c.Has(k) {
				return nil
			}
			v := c.Get(k)
			sub := v.Message()
			if err := o.PatchField(sub, fd, delta); err != nil {
				return fmt.Errorf("key %v: %w", k, err)
			}
			c.Set(k, protoreflect.ValueOfMessage(sub))
			return nil
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, c.Len())
	for _, k := range keys {
		leave := o.cursorEnter(mapKeyToPathEntry(k))
		before := Frame{Descriptor: vd, Value: c.Get(k)}
		err := op(k)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%v]: %w", k, err))
		} else if notify_leaf {
			o.cursorNotify(before, Frame{Descriptor: vd, Value: c.Get(k)}, entry)
		}
		leave()
	}
	if err := after(); err != nil {
		errs = append(errs, fmt.Errorf("clean up: %w", err))
	} else if notify_leaf {
		after_notify()
	}
	return errors.Join(errs...)
}

// mapKeysFromSegments converts target Segments to MapKey values.
func mapKeysFromSegments(targets []*dpb.Segment, kind protoreflect.Kind) ([]protoreflect.MapKey, error) {
	keys := make([]protoreflect.MapKey, 0, len(targets))
	for _, seg := range targets {
		var k protoreflect.MapKey
		switch seg.WhichKind() {
		case dpb.Segment_Name_case:
			switch kind {
			case protoreflect.StringKind:
				k = protoreflect.ValueOfString(seg.GetName()).MapKey()
			default:
				return nil, fmt.Errorf("string segment not valid for map key kind %s", kind)
			}
		case dpb.Segment_Index_case:
			idx := seg.GetIndex()
			switch kind {
			case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
				k = protoreflect.ValueOfInt32(int32(idx)).MapKey()
			case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
				k = protoreflect.ValueOfInt64(idx).MapKey()
			case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
				k = protoreflect.ValueOfUint32(uint32(idx)).MapKey()
			case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
				k = protoreflect.ValueOfUint64(uint64(idx)).MapKey()
			case protoreflect.BoolKind:
				k = protoreflect.ValueOfBool(idx != 0).MapKey()
			case protoreflect.StringKind:
				// allow integer segments for string maps
				k = protoreflect.ValueOfString(fmt.Sprintf("%d", idx)).MapKey()
			default:
				return nil, fmt.Errorf("index segment not valid for map key kind %s", kind)
			}
		case dpb.Segment_Field_case:
			var err error
			k, err = fieldSegmentToMapKey(seg.GetField(), kind)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported segment kind for map target: %v", seg.WhichKind())
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// fieldSegmentToMapKey converts a FieldSegment to a MapKey for the given key kind.
func fieldSegmentToMapKey(fs *dpb.FieldSegment, kind protoreflect.Kind) (protoreflect.MapKey, error) {
	if fs == nil {
		return protoreflect.MapKey{}, fmt.Errorf("nil field segment")
	}
	switch kind {
	case protoreflect.StringKind:
		if fs.HasName() && fs.GetName() != "" {
			return protoreflect.ValueOfString(fs.GetName()).MapKey(), nil
		}
		return protoreflect.ValueOfString(fmt.Sprintf("%d", fs.GetNumber())).MapKey(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(fs.GetNumber())).MapKey(), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(fs.GetNumber()).MapKey(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(fs.GetNumber())).MapKey(), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(uint64(fs.GetNumber())).MapKey(), nil
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(fs.GetNumber() != 0).MapKey(), nil
	default:
		return protoreflect.MapKey{}, fmt.Errorf("unsupported map key kind %s", kind)
	}
}
