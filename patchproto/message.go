package patchproto

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func (o PatchOption) patchMessage(c protoreflect.Message, _ protoreflect.FieldDescriptor, targets []*dpb.Segment, entry *dpb.Entry) error {
	if len(targets) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.PatchField(c, nil, entry.GetNest())
		}
		return nil
	}

	fields := c.Descriptor().Fields()

	notify_leaf := true
	op := func(fd protoreflect.FieldDescriptor) error { return nil }

	switch entry.WhichKind() {
	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			op = func(fd protoreflect.FieldDescriptor) error {
				c.Clear(fd)
				return nil
			}
		}

	case dpb.Entry_Test_case:
		val := entry.GetTest()
		op = func(fd protoreflect.FieldDescriptor) error {
			current := c.Get(fd)
			expected, err := valueToProtoValue(val, fd, o.Types)
			if err != nil {
				return fmt.Errorf("test: decode: %w", err)
			}
			if !protoValueEqual(current, expected, fd.Kind()) {
				return fmt.Errorf("test failed at %s", fd.FullName())
			}
			return nil
		}
		notify_leaf = false

	case dpb.Entry_Insert_case:
		val := entry.GetInsert()
		op = func(fd protoreflect.FieldDescriptor) error {
			if fd.HasPresence() && c.Has(fd) {
				return nil // already present — no-op for insert
			}
			v, err := valueToProtoValue(val, fd, o.Types)
			if err != nil {
				return fmt.Errorf("insert: decode: %w", err)
			}
			if v.IsValid() {
				c.Set(fd, v)
			}
			return nil
		}

	case dpb.Entry_Assign_case:
		val := entry.GetAssign()
		op = func(fd protoreflect.FieldDescriptor) error {
			v, err := valueToProtoValue(val, fd, o.Types)
			if err != nil {
				return fmt.Errorf("assign: decode: %w", err)
			}
			if v.IsValid() {
				c.Set(fd, v)
			} else {
				c.Clear(fd)
			}
			return nil
		}

	case dpb.Entry_Move_case:
		src := entry.GetMove()
		fd_src := findFieldByFieldSegment(fields, src)
		if fd_src == nil {
			return nil
		}
		v := c.Get(fd_src)
		hasSrc := c.Has(fd_src)
		cleared := false
		op = func(fd protoreflect.FieldDescriptor) error {
			if !hasSrc {
				c.Clear(fd)
			} else {
				w, err := cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}
				c.Set(fd, w)
			}
			if !cleared {
				c.Clear(fd_src)
				cleared = true
			}
			return nil
		}

	case dpb.Entry_Copy_case:
		src := entry.GetCopy()
		fd_src := findFieldByFieldSegment(fields, src)
		if fd_src == nil {
			return nil
		}
		hasSrc := c.Has(fd_src)
		v := c.Get(fd_src)
		op = func(fd protoreflect.FieldDescriptor) error {
			if !hasSrc {
				c.Clear(fd)
			} else {
				w, err := cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}
				c.Set(fd, w)
			}
			return nil
		}

	case dpb.Entry_Nest_case:
		notify_leaf = false
		delta := entry.GetNest()
		op = func(fd protoreflect.FieldDescriptor) error {
			kind := fd.Kind()
			switch {
			case fd.IsList():
				return o.PatchField(c.Mutable(fd).List(), fd, delta)
			case fd.IsMap():
				return o.PatchField(c.Mutable(fd).Map(), fd, delta)
			case kind == protoreflect.MessageKind || kind == protoreflect.GroupKind:
				return o.PatchField(c.Mutable(fd).Message(), fd, delta)
			default:
				return fmt.Errorf("field %s: nest cannot be applied to %q", fd.FullName(), kind)
			}
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(targets))
	for _, seg := range targets {
		fd := messageFieldBySeg(fields, seg)
		if fd == nil {
			continue
		}

		leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: string(fd.Name()), Index: int(fd.Number())})
		before := Frame{Descriptor: fd, Value: c.Get(fd)}
		err := op(fd)
		if err != nil {
			errs = append(errs, fmt.Errorf("field %s: %w", fd.FullName(), err))
		} else if notify_leaf {
			o.cursorNotify(before, Frame{Descriptor: fd, Value: c.Get(fd)}, entry)
		}
		leave()
	}
	return errors.Join(errs...)
}

// messageFieldBySeg resolves a Segment to a FieldDescriptor within a message.
func messageFieldBySeg(fields protoreflect.FieldDescriptors, seg *dpb.Segment) protoreflect.FieldDescriptor {
	switch seg.WhichKind() {
	case dpb.Segment_Name_case:
		return fields.ByName(protoreflect.Name(seg.GetName()))
	case dpb.Segment_Index_case:
		num := seg.GetIndex()
		if num <= 0 {
			return nil
		}
		return fields.ByNumber(protoreflect.FieldNumber(num))
	case dpb.Segment_Field_case:
		return findFieldByFieldSegment(fields, seg.GetField())
	}
	return nil
}
