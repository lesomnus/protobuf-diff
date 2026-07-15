package patchproto

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func (o PatchOption) patchMessage(c protoreflect.Message, fd protoreflect.FieldDescriptor, targets []*dpb.Segment, entry *dpb.Entry) error {
	if len(targets) == 0 {
		return o.patchMessageRoot(c, fd, entry)
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
		if fd_src.IsList() || fd_src.IsMap() {
			return fmt.Errorf("move source %s cannot be a repeated or map field", fd_src.FullName())
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
		if fd_src.IsList() || fd_src.IsMap() {
			return fmt.Errorf("copy source %s cannot be a repeated or map field", fd_src.FullName())
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

	// assign/insert/test/move/copy operate on a single scalar or message value;
	// applied to a repeated or map field they would panic in protoreflect. Only
	// remove (clear) and nest (recurse) are valid on those. Reject the rest with
	// a clear error, pointing at nest or a root (no-target) operation instead.
	singularOnly := false
	switch entry.WhichKind() {
	case dpb.Entry_Assign_case, dpb.Entry_Insert_case, dpb.Entry_Test_case,
		dpb.Entry_Move_case, dpb.Entry_Copy_case:
		singularOnly = true
	}

	errs := make([]error, 0, len(targets))
	for _, seg := range targets {
		fd := messageFieldBySeg(fields, seg)
		if fd == nil {
			continue
		}
		if singularOnly && (fd.IsList() || fd.IsMap()) {
			errs = append(errs, fmt.Errorf("field %s: %v cannot target a repeated or map field; use nest or a root operation", fd.FullName(), entry.WhichKind()))
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

// patchMessageRoot applies an entry with no targets to the message itself —
// the root, or the message reached by the entry's path. This is where
// whole-message operations (replace, clear, test) live.
func (o PatchOption) patchMessageRoot(c protoreflect.Message, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	switch entry.WhichKind() {
	case dpb.Entry_Nest_case:
		return o.PatchField(c, nil, entry.GetNest())

	case dpb.Entry_Remove_case:
		if !entry.GetRemove() {
			return nil
		}
		before := snapshotMessage(c)
		clearAllMessageFields(c)
		o.cursorNotify(Frame{Descriptor: fd, Value: before}, Frame{Descriptor: fd, Value: protoreflect.ValueOfMessage(c)}, entry)
		return nil

	case dpb.Entry_Test_case:
		val := entry.GetTest()
		if isClearValue(val) {
			if !messageIsEmpty(c) {
				return fmt.Errorf("test failed at root: message is not empty")
			}
			return nil
		}
		if val.WhichKind() != dpb.Value_M_case {
			return fmt.Errorf("test at message root requires a message value, got %v", val.WhichKind())
		}
		expected := c.New()
		if err := applyStructToMessage(val.GetM(), expected, o.Types); err != nil {
			return fmt.Errorf("test: decode: %w", err)
		}
		if !proto.Equal(c.Interface(), expected.Interface()) {
			return fmt.Errorf("test failed at root")
		}
		return nil

	case dpb.Entry_Assign_case:
		val := entry.GetAssign()
		if !isClearValue(val) && val.WhichKind() != dpb.Value_M_case {
			return fmt.Errorf("assign at message root requires a message value, got %v", val.WhichKind())
		}
		before := snapshotMessage(c)
		clearAllMessageFields(c)
		if !isClearValue(val) {
			if err := applyStructToMessage(val.GetM(), c, o.Types); err != nil {
				return fmt.Errorf("assign: decode: %w", err)
			}
		}
		o.cursorNotify(Frame{Descriptor: fd, Value: before}, Frame{Descriptor: fd, Value: protoreflect.ValueOfMessage(c)}, entry)
		return nil

	case dpb.Entry_Insert_case:
		if !messageIsEmpty(c) {
			return nil // present — no-op for insert
		}
		val := entry.GetInsert()
		if isClearValue(val) {
			return nil
		}
		if val.WhichKind() != dpb.Value_M_case {
			return fmt.Errorf("insert at message root requires a message value, got %v", val.WhichKind())
		}
		before := snapshotMessage(c)
		if err := applyStructToMessage(val.GetM(), c, o.Types); err != nil {
			return fmt.Errorf("insert: decode: %w", err)
		}
		o.cursorNotify(Frame{Descriptor: fd, Value: before}, Frame{Descriptor: fd, Value: protoreflect.ValueOfMessage(c)}, entry)
		return nil

	default:
		return fmt.Errorf("unsupported root op for message: %q", entry.WhichKind())
	}
}

// snapshotMessage returns a deep copy of c wrapped as a protoreflect.Value, for
// use as the "before" frame of a root-level notification.
func snapshotMessage(c protoreflect.Message) protoreflect.Value {
	return protoreflect.ValueOfMessage(proto.Clone(c.Interface()).ProtoReflect())
}

// clearAllMessageFields clears every populated field of c.
func clearAllMessageFields(c protoreflect.Message) {
	var fds []protoreflect.FieldDescriptor
	c.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		fds = append(fds, fd)
		return true
	})
	for _, fd := range fds {
		c.Clear(fd)
	}
}

// messageIsEmpty reports whether c has no populated fields.
func messageIsEmpty(c protoreflect.Message) bool {
	empty := true
	c.Range(func(protoreflect.FieldDescriptor, protoreflect.Value) bool {
		empty = false
		return false
	})
	return empty
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
