package patchjson

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"google.golang.org/protobuf/encoding/protowire"
)

func decodeStringKeys(data []byte) ([]string, error) {
	var keys []string
	for len(data) > 0 {
		s, n := protowire.ConsumeString(data)
		if n < 0 {
			return nil, fmt.Errorf("invalid string key encoding: %w", protowire.ParseError(n))
		}
		keys = append(keys, s)
		data = data[n:]
	}
	return keys, nil
}

func (o PatchOption) patchMap(m map[string]any, entry *dpb.Entry) error {
	keys, err := decodeStringKeys(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	check := func(k string) bool {
		_, exists := m[k]
		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	notify_leaf := true
	op := func(k string) error { return nil }
	after := func() error { return nil }
	after_notify := func() {}

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(k string) error {
				delete(m, k)
				return nil
			}
		}

	case dpb.Entry_Assigned_case:
		var val any
		var decoded bool
		op = func(k string) error {
			if !check(k) {
				return nil
			}
			if !decoded {
				var err error
				val, err = decodeValue(entry.GetAssigned())
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
				decoded = true
			}
			m[k] = val
			return nil
		}

	case dpb.Entry_Merged_case:
		var patch map[string]any
		var decoded bool
		op = func(k string) error {
			if !check(k) {
				return nil
			}
			child, exists := m[k]
			if !exists {
				return nil
			}
			childMap, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("merged target must be an object, got %T", child)
			}
			if !decoded {
				v, err := decodeValue(entry.GetMerged())
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
				patch, ok = v.(map[string]any)
				if !ok {
					return fmt.Errorf("merged value must be an object, got %T", v)
				}
				decoded = true
			}
			for pk, pv := range patch {
				childMap[pk] = pv
			}
			return nil
		}

	case dpb.Entry_Copied_case:
		src := ref.DecodeString(entry.GetCopied())
		srcVal, exists := m[src]
		if !exists {
			op = func(k string) error {
				if !check(k) {
					return nil
				}
				delete(m, k)
				return nil
			}
		} else {
			op = func(k string) error {
				if !check(k) {
					return nil
				}
				m[k] = srcVal
				return nil
			}
		}

	case dpb.Entry_Scattered_case:
		src := ref.DecodeString(entry.GetScattered())
		srcVal, exists := m[src]
		if !exists {
			op = func(k string) error {
				if !check(k) {
					return nil
				}
				delete(m, k)
				return nil
			}
		} else {
			op = func(k string) error {
				if !check(k) {
					return nil
				}
				m[k] = srcVal
				return nil
			}
			after = func() error {
				delete(m, src)
				return nil
			}
			after_notify = func() {
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: src})
				o.cursorNotify(entry)
				leave()
			}
		}

	case dpb.Entry_Swapped_case:
		src := ref.DecodeString(entry.GetSwapped())
		tmp := m[src]
		op = func(k string) error {
			w := m[k]
			m[k] = tmp
			tmp = w
			return nil
		}
		after = func() error {
			m[src] = tmp
			return nil
		}
		after_notify = func() {
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: src})
			o.cursorNotify(entry)
			leave()
		}

	case dpb.Entry_Nested_case:
		notify_leaf = false
		delta := entry.GetNested()
		op = func(k string) error {
			if !check(k) {
				return nil
			}
			child, exists := m[k]
			if !exists {
				return nil
			}
			childSet := func(v any) { m[k] = v }
			return o.patchField(child, childSet, delta)
		}

	case dpb.Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(keys))
	for _, k := range keys {
		leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryField, Key: k})
		err := op(k)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s]: %w", k, err))
		} else if notify_leaf {
			o.cursorNotify(entry)
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
