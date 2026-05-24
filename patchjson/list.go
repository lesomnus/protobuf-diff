package patchjson

import (
	"fmt"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func (o PatchOption) patchList(list []any, set func(any), entry *dpb.Entry) error {
	indices, err := target.DecodeIndices(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(indices) == 0 {
		return nil
	}

	l := len(list)
	if l == 0 && !entry.GetNoUpdate() {
		return nil
	}

	normal := func(i int) (int, bool) {
		if i < 0 {
			i = l + i
		}
		if i < 0 || i >= l {
			return 0, false
		}
		return i, true
	}

	splice := func(val any) {
		insert_before := make([]int, 0, len(indices))
		pushbacks := 0
		for _, i := range indices {
			if i < -1-l {
				continue
			}
			if i < -1 {
				i = l + i + 1
			}
			if i == -1 || i == l {
				pushbacks++
				continue
			}
			if i > l {
				continue
			}
			insert_before = append(insert_before, i)
		}
		slices.Sort(insert_before)

		w := make([]any, 0, l+len(insert_before)+pushbacks)
		j := 0
		for i, u := range list {
			for j < len(insert_before) && insert_before[j] == i {
				w = append(w, val)
				j++
			}
			w = append(w, u)
		}
		for range pushbacks {
			w = append(w, val)
		}
		list = w
		set(list)
	}

	// notify_insert fires hooks for each user-specified insertion index.
	notify_insert := func(val any) {
		for _, i := range indices {
			if i < -1-l || i > l {
				continue
			}
			idx := i
			if i < -1 {
				idx = l + i + 1
			} else if i == -1 {
				idx = l
			}
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: idx})
			o.cursorNotify(Frame{}, Frame{Value: val}, entry)
			leave()
		}
	}

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		if !entry.GetDeleted() {
			break
		}
		targets_set := make(map[int]struct{}, len(indices))
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			targets_set[i] = struct{}{}
		}

		w := make([]any, 0, l)
		for i, u := range list {
			if _, ok := targets_set[i]; ok {
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				o.cursorNotify(Frame{Value: u}, Frame{}, entry)
				leave()
				continue
			}
			w = append(w, u)
		}
		list = w
		set(list)

	case dpb.Entry_Assigned_case:
		val, err := decodeValue(entry.GetAssigned())
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}

		if entry.GetNoUpdate() {
			splice(val)
			notify_insert(val)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				before := Frame{Value: list[i]}
				list[i] = val
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				o.cursorNotify(before, Frame{Value: list[i]}, entry)
				leave()
			}
		}

	case dpb.Entry_Copied_case:
		k, err := ref.DecodeIndex(entry.GetCopied())
		if err != nil {
			return fmt.Errorf("copy: decode source index: %w", err)
		}
		k, ok := normal(k)
		if !ok {
			return nil
		}
		val := list[k]

		if entry.GetNoUpdate() {
			splice(val)
			notify_insert(val)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				before := Frame{Value: list[i]}
				list[i] = val
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				o.cursorNotify(before, Frame{Value: list[i]}, entry)
				leave()
			}
		}

	case dpb.Entry_Scattered_case:
		k, err := ref.DecodeIndex(entry.GetScattered())
		if err != nil {
			return fmt.Errorf("scatter: decode source index: %w", err)
		}
		k, ok := normal(k)
		if !ok {
			return nil
		}
		val := list[k]

		if entry.GetNoUpdate() {
			insert_before := make([]int, 0, len(indices))
			pushbacks := 0
			offset := 0
			for _, i := range indices {
				if i < -1-l {
					continue
				}
				if i < -1 {
					i = l + i + 1
				}
				if i == -1 || i == l {
					pushbacks++
					continue
				}
				if i > l {
					continue
				}
				if i < k {
					offset++
				}
				insert_before = append(insert_before, i)
			}
			slices.Sort(insert_before)

			w := make([]any, 0, l+len(insert_before)+pushbacks)
			j := 0
			for i, u := range list {
				for j < len(insert_before) && insert_before[j] == i {
					w = append(w, val)
					j++
				}
				w = append(w, u)
			}
			for range pushbacks {
				w = append(w, val)
			}

			adjustedK := k + offset
			final := make([]any, 0, len(w)-1)
			for i, u := range w {
				if i == adjustedK {
					continue
				}
				final = append(final, u)
			}
			list = final
			set(list)

			notify_insert(val)
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: k})
			o.cursorNotify(Frame{Value: val}, Frame{}, entry)
			leave()
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				before := Frame{Value: list[i]}
				list[i] = val
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				o.cursorNotify(before, Frame{Value: list[i]}, entry)
				leave()
			}
			w := make([]any, 0, l-1)
			for i, u := range list {
				if i == k {
					continue
				}
				w = append(w, u)
			}
			list = w
			set(list)

			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: k})
			o.cursorNotify(Frame{Value: val}, Frame{}, entry)
			leave()
		}

	case dpb.Entry_Swapped_case:
		k, err := ref.DecodeIndex(entry.GetSwapped())
		if err != nil {
			return fmt.Errorf("swap: decode source index: %w", err)
		}
		k, ok := normal(k)
		if !ok {
			return nil
		}

		kBefore := list[k]
		tmp := list[k]
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			before := Frame{Value: list[i]}
			list[i], tmp = tmp, list[i]
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
			o.cursorNotify(before, Frame{Value: list[i]}, entry)
			leave()
		}
		list[k] = tmp
		leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: k})
		o.cursorNotify(Frame{Value: kBefore}, Frame{Value: list[k]}, entry)
		leave()

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()

		if entry.GetNoUpdate() {
			var newElem any = map[string]any{}
			newElemSet := func(v any) { newElem = v }
			if err := o.patchField(newElem, newElemSet, delta); err != nil {
				return err
			}
			splice(newElem)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				child := list[i]
				childSet := func(v any) {
					list[i] = v
				}
				if err := o.patchField(child, childSet, delta); err != nil {
					leave()
					return fmt.Errorf("[%d]: %w", i, err)
				}
				leave()
			}
		}

	case dpb.Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	return nil
}
