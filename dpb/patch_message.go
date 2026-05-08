package dpb

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func (o PatchOption) patchMessage(c protoreflect.Message, entry *Entry) error {
	targets, err := target.DecodeFieldNumbers(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	fields := c.Descriptor().Fields()
	fd0 := fields.ByNumber(targets[0])
	if fd0 == nil {
		return fmt.Errorf("field [%d] not found in %q", targets[0], c.Descriptor().FullName())
	}

	op := func(fd protoreflect.FieldDescriptor) error {
		return nil
	}
	check := func(fd protoreflect.FieldDescriptor) bool {
		exists := true
		if fd.HasPresence() {
			exists = c.Has(fd)
		}

		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	switch entry.WhichKind() {
	case Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(fd protoreflect.FieldDescriptor) error {
				c.Clear(fd)
				return nil
			}
		}

	case Entry_Assigned_case:
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}

			v, err := decodeValue(entry.GetAssigned(), fd)
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}

			c.Set(fd, v)
			return nil
		}

	case Entry_Merged_case:
		var v proto.Message
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}
			if fd.Kind() != protoreflect.MessageKind {
				return fmt.Errorf("field must be message type %q", fd.FullName())
			}
			if v == nil {
				m := dynamicpb.NewMessageType(fd.Message()).New()
				v = m.Interface()
				if err := proto.Unmarshal(entry.GetMerged(), v); err != nil {
					return fmt.Errorf("unmarshal: %w", err)
				}
			}

			w := c.Get(fd)
			proto.Merge(w.Message().Interface(), v)
			return nil
		}

	case Entry_Copied_case:
		k, err := ref.DecodeField(entry.GetCopied())
		if err != nil {
			return fmt.Errorf("copy: unmarshal source field number: %w", err)
		}

		fd_src := fields.ByNumber(k)
		if !c.Has(fd_src) {
			// Source field is not set, so clear target fields without setting.
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				c.Clear(fd)
				return nil
			}
		} else {
			v := c.Get(fd_src)
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				w, err := cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}

				c.Set(fd, w)
				return nil
			}
		}

	case Entry_Scattered_case:
		k, err := ref.DecodeField(entry.GetScattered())
		if err != nil {
			return fmt.Errorf("scatter: unmarshal source field number: %w", err)
		}

		fd_src := fields.ByNumber(k)
		if !c.Has(fd_src) {
			// Source field is not set, so clear target fields without setting.
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				c.Clear(fd)
				return nil
			}
		} else {
			v := c.Get(fd_src)
			done := false
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				w, err := cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}

				c.Set(fd, w)
				if !done {
					c.Clear(fd_src)
					done = true
				}
				return nil
			}
		}

	case Entry_Swapped_case:
		target, err := ref.DecodeField(entry.GetSwapped())
		if err != nil {
			return fmt.Errorf("swap: unmarshal target field number: %w", err)
		}

		op = func(fd_src protoreflect.FieldDescriptor) error {
			fd_dst := fields.ByNumber(target)
			if fd_dst == nil {
				return nil
			}

			ka := fd_src.Kind()
			kb := fd_dst.Kind()

			va, err := cast(c.Get(fd_src), ka, kb)
			if err != nil {
				return fmt.Errorf("cast src: %w", err)
			}

			vb, err := cast(c.Get(fd_dst), kb, ka)
			if err != nil {
				return fmt.Errorf("cast dst: %w", err)
			}

			c.Set(fd_src, vb)
			c.Set(fd_dst, va)
			return nil
		}

	case Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	case Entry_Nested_case:
		delta := entry.GetNested()
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}

			kind := fd.Kind()
			switch {
			case fd.IsList():
				sub := c.Mutable(fd).List()
				if err := o.PatchField(sub, fd, delta); err != nil {
					return fmt.Errorf("field %s: %w", fd.FullName(), err)
				}

			case fd.IsMap():
				sub := c.Mutable(fd).Map()
				if err := o.PatchField(sub, fd, delta); err != nil {
					return fmt.Errorf("field %s: %w", fd.FullName(), err)
				}

			case kind == protoreflect.MessageKind || kind == protoreflect.GroupKind:
				sub := c.Mutable(fd).Message()
				return o.PatchField(sub, fd, delta)

			default:
				return fmt.Errorf("field %s: nested delta cannot be applied to %q", fd.FullName(), kind.String())
			}
			return nil
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(targets))
	for _, i := range targets {
		fd := fields.ByNumber(i)
		if fd == nil {
			continue
		}

		if err := op(fd); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}
