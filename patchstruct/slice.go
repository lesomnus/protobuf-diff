package patchstruct

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func (o PatchOption) patchSlice(v reflect.Value, entry *dpb.Entry) error {
	indices, err := target.DecodeIndices(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(indices) == 0 {
		return nil
	}

	l := v.Len()
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

	// splice inserts val before each index in indices (or appends for -1/l).
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
	case dpb.Entry_Deleted_case:
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

	case dpb.Entry_Assigned_case:
		val, err := decodeValue(entry.GetAssigned(), v.Type().Elem())
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}

		elem := reflect.New(v.Type().Elem()).Elem()
		setField(elem, val)
		if entry.GetNoUpdate() {
			splice(elem)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				v.Index(i).Set(elem)
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

		elem := v.Index(k)
		if entry.GetNoUpdate() {
			splice(elem)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				v.Index(i).Set(elem)
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

		elem := reflect.New(v.Type().Elem()).Elem()
		elem.Set(v.Index(k))

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

			w := reflect.MakeSlice(v.Type(), 0, l+len(insert_before)+pushbacks)
			j := 0
			for i := range l {
				for j < len(insert_before) && insert_before[j] == i {
					w = reflect.Append(w, elem)
					j++
				}
				w = reflect.Append(w, v.Index(i))
			}
			for range pushbacks {
				w = reflect.Append(w, elem)
			}

			// Remove source element (adjusted for inserted elements).
			wl := w.Len()
			adjustedK := k + offset
			final := reflect.MakeSlice(v.Type(), 0, wl-1)
			for i := range wl {
				if i == adjustedK {
					continue
				}
				final = reflect.Append(final, w.Index(i))
			}
			v.Set(final)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				v.Index(i).Set(elem)
			}

			// Remove source element by rebuilding without index k.
			w := reflect.MakeSlice(v.Type(), 0, l-1)
			for i := range l {
				if i == k {
					continue
				}
				w = reflect.Append(w, v.Index(i))
			}
			v.Set(w)
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

		tmp := reflect.New(v.Type().Elem()).Elem()
		tmp.Set(v.Index(k))
		for _, i := range indices {
			i, ok := normal(i)
			if !ok {
				continue
			}
			w := reflect.New(v.Type().Elem()).Elem()
			w.Set(v.Index(i))
			v.Index(i).Set(tmp)
			tmp = w
		}
		v.Index(k).Set(tmp)

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()
		et := v.Type().Elem()
		for et.Kind() == reflect.Pointer {
			et = et.Elem()
		}
		if et.Kind() != reflect.Struct {
			return fmt.Errorf("nested deltas for slices can only be applied to structs, got %v", et.Kind())
		}

		if entry.GetNoUpdate() {
			newElem := reflect.New(et).Elem()
			if err := o.PatchField(newElem, delta); err != nil {
				return err
			}
			final := reflect.New(v.Type().Elem()).Elem()
			setField(final, newElem)
			splice(final)
		} else {
			for _, i := range indices {
				i, ok := normal(i)
				if !ok {
					continue
				}
				elem := v.Index(i)
				ev := elem
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
		}
	}

	return nil
}
