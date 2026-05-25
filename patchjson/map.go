package patchjson

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func decodeMapTargets(targets []*dpb.Segment) []string {
	var keys []string
	for _, seg := range targets {
		switch seg.WhichKind() {
		case dpb.Segment_Name_case:
			keys = append(keys, seg.GetName())
		case dpb.Segment_Field_case:
			if fs := seg.GetField(); fs != nil && fs.HasName() {
				keys = append(keys, fs.GetName())
			}
		}
	}
	return keys
}

func (o PatchOption) patchMap(m map[string]any, entry *dpb.Entry) error {
	keys := decodeMapTargets(entry.GetTargets())
	if len(keys) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.patchField(m, func(any) {}, entry.GetNest())
		}
		return nil
	}

	notify_leaf := true
	op := func(k string) error { return nil }
	after := func() error { return nil }
	after_notify := func() {}

	switch entry.WhichKind() {
	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			op = func(k string) error {
				delete(m, k)
				return nil
			}
		}

	case dpb.Entry_Test_case:
		notify_leaf = false
		expected := dpbValueToAny(entry.GetTest())
		op = func(k string) error {
			if m[k] != expected {
				return fmt.Errorf("test failed at key %q", k)
			}
			return nil
		}

	case dpb.Entry_Insert_case:
		val := dpbValueToAny(entry.GetInsert())
		op = func(k string) error {
			if _, exists := m[k]; !exists {
				m[k] = val
			}
			return nil
		}

	case dpb.Entry_Assign_case:
		val := dpbValueToAny(entry.GetAssign())
		op = func(k string) error {
			m[k] = val
			return nil
		}

	case dpb.Entry_Move_case:
		src := entry.GetMove().GetName()
		src_v, exists := m[src]
		src_before := Frame{Value: src_v}
		if !exists {
			op = func(k string) error {
				delete(m, k)
				return nil
			}
		} else {
			op = func(k string) error {
				m[k] = src_v
				return nil
			}
			after = func() error {
				delete(m, src)
				return nil
			}
			after_notify = func() {
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: src})
				o.cursorNotify(src_before, Frame{}, entry)
				leave()
			}
		}

	case dpb.Entry_Copy_case:
		src := entry.GetCopy().GetName()
		src_v, exists := m[src]
		if !exists {
			op = func(k string) error {
				delete(m, k)
				return nil
			}
		} else {
			op = func(k string) error {
				m[k] = src_v
				return nil
			}
		}

	case dpb.Entry_Nest_case:
		notify_leaf = false
		delta := entry.GetNest()
		op = func(k string) error {
			child, exists := m[k]
			if !exists {
				return nil
			}
			childSet := func(v any) { m[k] = v }
			return o.patchField(child, childSet, delta)
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(keys))
	for _, k := range keys {
		leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: k})
		before := Frame{Value: m[k]}
		err := op(k)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s]: %w", k, err))
		} else if notify_leaf {
			o.cursorNotify(before, Frame{Value: m[k]}, entry)
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
