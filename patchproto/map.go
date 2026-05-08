package patchproto

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func (o PatchOption) patchMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	kd := fd.MapKey()
	keys, err := target.DecodeKeys(entry.GetTargets(), kd.Kind())
	if err != nil {
		return fmt.Errorf("decode keys: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	vd := fd.MapValue()
	op := func(k protoreflect.MapKey) error { return nil }
	after := func() error { return nil }
	check := func(k protoreflect.MapKey) bool {
		exists := c.Has(k)
		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(k protoreflect.MapKey) error {
				c.Clear(k)
				return nil
			}
		}

	case dpb.Entry_Assigned_case:
		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if !check(k) {
				return nil
			}
			if !v.IsValid() {
				var err error
				v, err = decodeValue(entry.GetAssigned(), vd)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
			}

			c.Set(k, v)
			return nil
		}

	case dpb.Entry_Merged_case:
		if vd.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("value must be message type %q", vd.FullName())
		}

		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if !check(k) {
				return nil
			}
			if !v.IsValid() {
				var err error
				v, err = decodeValue(entry.GetAssigned(), vd)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
			}

			w := c.Get(k)
			proto.Merge(w.Message().Interface(), v.Message().Interface())
			return nil
		}

	case dpb.Entry_Copied_case:
		src, err := ref.DecodeKey(entry.GetCopied(), kd.Kind())
		if err != nil {
			return fmt.Errorf("copy: unmarshal source key: %w", err)
		}
		if !c.Has(src) {
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				if !v.IsValid() {
					v = c.Get(src)
				}
				c.Set(k, v)
				return nil
			}
		}

	case dpb.Entry_Scattered_case:
		src, err := ref.DecodeKey(entry.GetScattered(), kd.Kind())
		if err != nil {
			return fmt.Errorf("scatter: unmarshal source key: %w", err)
		}
		if !c.Has(src) {
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				if !v.IsValid() {
					v = c.Get(src)
				}
				c.Set(k, v)
				return nil
			}
		}
		after = func() error {
			c.Clear(src)
			return nil
		}

	case dpb.Entry_Swapped_case:
		src, err := ref.DecodeKey(entry.GetSwapped(), kd.Kind())
		if err != nil {
			return fmt.Errorf("swap: unmarshal source key: %w", err)
		}

		v := c.Get(src)
		op = func(k protoreflect.MapKey) error {
			w := c.Get(k)
			c.Set(k, v)
			v = w
			return nil
		}
		after = func() error {
			c.Set(src, v)
			return nil
		}

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()
		kind := vd.Kind()
		if !(kind == protoreflect.MessageKind || kind == protoreflect.GroupKind) {
			return fmt.Errorf("nested deltas for maps can only be applied to message values, got %q", kind.String())
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

	case dpb.Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, c.Len())
	for _, k := range keys {
		if err := op(k); err != nil {
			errs = append(errs, fmt.Errorf("[%v]: %w", k, err))
		}
	}
	if err := after(); err != nil {
		errs = append(errs, fmt.Errorf("clean up: %w", err))
	}
	return errors.Join(errs...)
}
