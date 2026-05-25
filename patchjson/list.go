package patchjson

import (
	"fmt"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func decodeListTargets(targets []*dpb.Segment) []int {
	var indices []int
	for _, seg := range targets {
		if seg.WhichKind() == dpb.Segment_Index_case {
			indices = append(indices, int(seg.GetIndex()))
		}
	}
	return indices
}

func (o PatchOption) patchList(list []any, set func(any), entry *dpb.Entry) error {
	indices := decodeListTargets(entry.GetTargets())
	if len(indices) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.patchField(list, set, entry.GetNest())
		}
		return nil
	}

	l := len(list)

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
	case dpb.Entry_Remove_case:
		if !entry.GetRemove() {
			return nil
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

	case dpb.Entry_Test_case:
		val := dpbValueToAny(entry.GetTest())
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			if list[i] != val {
				return fmt.Errorf("test failed at index %d", i)
			}
		}

	case dpb.Entry_Insert_case:
		val := dpbValueToAny(entry.GetInsert())
		splice(val)
		notify_insert(val)

	case dpb.Entry_Assign_case:
		val := dpbValueToAny(entry.GetAssign())
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

	case dpb.Entry_Copy_case:
		k := int(entry.GetCopy().GetNumber())
		k, ok := normal(k)
		if !ok {
			return nil
		}
		val := list[k]
		splice(val)
		notify_insert(val)

	case dpb.Entry_Move_case:
		k := int(entry.GetMove().GetNumber())
		k, ok := normal(k)
		if !ok {
			return nil
		}
		val := list[k]

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

	case dpb.Entry_Nest_case:
		delta := entry.GetNest()
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
			child := list[i]
			childSet := func(v any) { list[i] = v }
			if err := o.patchField(child, childSet, delta); err != nil {
				leave()
				return fmt.Errorf("[%d]: %w", i, err)
			}
			leave()
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	return nil
}
