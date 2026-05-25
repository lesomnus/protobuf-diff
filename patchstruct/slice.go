package patchstruct

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func decodeSliceTargets(targets []*dpb.Segment) []int {
	var indices []int
	for _, seg := range targets {
		if seg.WhichKind() == dpb.Segment_Index_case {
			indices = append(indices, int(seg.GetIndex()))
		}
	}
	return indices
}

func (o PatchOption) patchSlice(v reflect.Value, entry *dpb.Entry) error {
	indices := decodeSliceTargets(entry.GetTargets())
	if len(indices) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.PatchField(v, entry.GetNest())
		}
		return nil
	}

	l := v.Len()

	normal := func(i int) (int, bool) {
		if i < 0 {
			i = l + i
		}
		if i < 0 || i >= l {
			return 0, false
		}
		return i, true
	}

	splice := func(val reflect.Value) {
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

		w := reflect.MakeSlice(v.Type(), 0, l+len(insert_before)+pushbacks)
		j := 0
		for i := range l {
			for j < len(insert_before) && insert_before[j] == i {
				w = reflect.Append(w, val)
				j++
			}
			w = reflect.Append(w, v.Index(i))
		}
		for range pushbacks {
			w = reflect.Append(w, val)
		}
		v.Set(w)
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

		w := reflect.MakeSlice(v.Type(), 0, l)
		for i := range l {
			if _, ok := targets_set[i]; ok {
				continue
			}
			w = reflect.Append(w, v.Index(i))
		}
		v.Set(w)

	case dpb.Entry_Test_case:
		val, err := decodeValue(entry.GetTest(), v.Type().Elem())
		if err != nil {
			return fmt.Errorf("decode test value: %w", err)
		}
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			if v.Index(i).Interface() != val.Interface() {
				return fmt.Errorf("test failed at index %d", i)
			}
		}

	case dpb.Entry_Insert_case:
		val, err := decodeValue(entry.GetInsert(), v.Type().Elem())
		if err != nil {
			return fmt.Errorf("decode insert value: %w", err)
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		setField(elem, val)
		splice(elem)

	case dpb.Entry_Assign_case:
		val, err := decodeValue(entry.GetAssign(), v.Type().Elem())
		if err != nil {
			return fmt.Errorf("decode assign value: %w", err)
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		setField(elem, val)
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			v.Index(i).Set(elem)
		}

	case dpb.Entry_Copy_case:
		k := int(entry.GetCopy().GetNumber())
		k, ok := normal(k)
		if !ok {
			return nil
		}
		splice(v.Index(k))

	case dpb.Entry_Move_case:
		k := int(entry.GetMove().GetNumber())
		k, ok := normal(k)
		if !ok {
			return nil
		}

		val := reflect.New(v.Type().Elem()).Elem()
		val.Set(v.Index(k))

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

		w := reflect.MakeSlice(v.Type(), 0, l+len(insert_before)+pushbacks)
		j := 0
		for i := range l {
			for j < len(insert_before) && insert_before[j] == i {
				w = reflect.Append(w, val)
				j++
			}
			w = reflect.Append(w, v.Index(i))
		}
		for range pushbacks {
			w = reflect.Append(w, val)
		}

		adjustedK := k + offset
		wl := w.Len()
		final := reflect.MakeSlice(v.Type(), 0, wl-1)
		for i := range wl {
			if i == adjustedK {
				continue
			}
			final = reflect.Append(final, w.Index(i))
		}
		v.Set(final)

	case dpb.Entry_Nest_case:
		delta := entry.GetNest()
		et := v.Type().Elem()
		for et.Kind() == reflect.Pointer {
			et = et.Elem()
		}
		if et.Kind() != reflect.Struct {
			return fmt.Errorf("nested deltas for slices can only be applied to structs, got %v", et.Kind())
		}

		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			ev := v.Index(i)
			for ev.Kind() == reflect.Pointer {
				if ev.IsNil() {
					ev.Set(reflect.New(ev.Type().Elem()))
				}
				ev = ev.Elem()
			}
			if err := o.PatchField(ev, delta); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	return nil
}
