package patchproto

import (
	"fmt"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func (o PatchOption) patchList(c protoreflect.List, fd protoreflect.FieldDescriptor, targets []*dpb.Segment, entry *dpb.Entry) error {
	if len(targets) == 0 {
		return o.patchListRoot(c, fd, entry)
	}

	l := c.Len()
	indices := expandListTargets(targets, l)

	normalize := func(i int) (int, bool) {
		if i < 0 {
			i = l + i
		}
		if i < 0 || i >= l {
			return 0, false
		}
		return i, true
	}

	// splice inserts v at each target position (insert mode).
	splice := func(v protoreflect.Value) {
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
		us := make([]protoreflect.Value, l)
		for i := range l {
			us[i] = c.Get(i)
		}
		c.Truncate(0)
		j := 0
		for i, u := range us {
			for j < len(insert_before) && insert_before[j] == i {
				c.Append(v)
				j++
			}
			c.Append(u)
		}
		for range pushbacks {
			c.Append(v)
		}
	}

	notify_insert := func(v protoreflect.Value) {
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
			o.cursorNotify(Frame{Descriptor: fd}, Frame{Descriptor: fd, Value: v}, entry)
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
			if n, ok := normalize(i); ok {
				targets_set[n] = struct{}{}
			}
		}
		j := 0
		for i := range l {
			if _, ok := targets_set[i]; ok {
				leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i})
				o.cursorNotify(Frame{Descriptor: fd, Value: c.Get(i)}, Frame{Descriptor: fd}, entry)
				leave()
				continue
			}
			c.Set(j, c.Get(i))
			j++
		}
		c.Truncate(j)

	case dpb.Entry_Test_case:
		val := entry.GetTest()
		for _, i := range indices {
			n, ok := normalize(i)
			if !ok {
				continue
			}
			expected, err := valueToProtoValue(val, fd, o.Types)
			if err != nil {
				return fmt.Errorf("test: decode: %w", err)
			}
			if !protoValueEqual(c.Get(n), expected, fd.Kind()) {
				return fmt.Errorf("test failed at index %d", n)
			}
		}

	case dpb.Entry_Insert_case:
		val := entry.GetInsert()
		v, err := valueToProtoValue(val, fd, o.Types)
		if err != nil {
			return fmt.Errorf("insert: decode: %w", err)
		}
		splice(v)
		notify_insert(v)

	case dpb.Entry_Assign_case:
		val := entry.GetAssign()
		v, err := valueToProtoValue(val, fd, o.Types)
		if err != nil {
			return fmt.Errorf("assign: decode: %w", err)
		}
		for _, i := range indices {
			n, ok := normalize(i)
			if !ok {
				continue
			}
			before := Frame{Descriptor: fd, Value: c.Get(n)}
			c.Set(n, v)
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: n})
			o.cursorNotify(before, Frame{Descriptor: fd, Value: c.Get(n)}, entry)
			leave()
		}

	case dpb.Entry_Move_case:
		src := entry.GetMove()
		k := int(src.GetNumber())
		k, ok := normalize(k)
		if !ok {
			return nil
		}
		v := c.Get(k)

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

		us := make([]protoreflect.Value, l)
		for i := range l {
			us[i] = c.Get(i)
		}
		c.Truncate(0)
		j := 0
		for i, u := range us {
			for j < len(insert_before) && insert_before[j] == i {
				c.Append(v)
				j++
			}
			c.Append(u)
		}
		for range pushbacks {
			c.Append(v)
		}

		newLen := l + j + pushbacks
		for i := k + offset; i < newLen-1; i++ {
			c.Set(i, c.Get(i+1))
		}
		notify_insert(v)
		leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: k})
		o.cursorNotify(Frame{Descriptor: fd, Value: v}, Frame{Descriptor: fd}, entry)
		leave()
		c.Truncate(newLen - 1)

	case dpb.Entry_Copy_case:
		src := entry.GetCopy()
		k := int(src.GetNumber())
		k, ok := normalize(k)
		if !ok {
			return nil
		}
		v := c.Get(k)
		splice(v)
		notify_insert(v)

	case dpb.Entry_Nest_case:
		delta := entry.GetNest()
		kind := fd.Kind()
		if !(kind == protoreflect.MessageKind || kind == protoreflect.GroupKind) {
			return fmt.Errorf("nest for lists requires message element kind, got %q", kind)
		}
		for _, i := range indices {
			n, ok := normalize(i)
			if !ok {
				continue
			}
			leave := o.cursorEnter(dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: n})
			sub := c.Get(n).Message()
			if err := o.PatchField(sub, fd, delta); err != nil {
				leave()
				return fmt.Errorf("[%d]: %w", n, err)
			}
			c.Set(n, protoreflect.ValueOfMessage(sub))
			leave()
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	return nil
}

// patchListRoot applies an entry with no targets to the list itself — the list
// reached by the entry's path. Bulk operations (replace, clear, append, test)
// operate on the whole list. Cursor notifications are not emitted for these bulk
// operations.
func (o PatchOption) patchListRoot(c protoreflect.List, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	switch entry.WhichKind() {
	case dpb.Entry_Nest_case:
		return o.PatchField(c, fd, entry.GetNest())

	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			c.Truncate(0)
		}
		return nil

	case dpb.Entry_Assign_case:
		val := entry.GetAssign()
		if isClearValue(val) {
			c.Truncate(0)
			return nil
		}
		if val.WhichKind() != dpb.Value_L_case {
			return fmt.Errorf("assign at list root requires a list value, got %v", val.WhichKind())
		}
		c.Truncate(0)
		if err := appendDpbValues(c, val.GetL().GetValues(), fd, o.Types); err != nil {
			return fmt.Errorf("assign: %w", err)
		}
		return nil

	case dpb.Entry_Insert_case:
		val := entry.GetInsert()
		if isClearValue(val) {
			return nil
		}
		if val.WhichKind() != dpb.Value_L_case {
			return fmt.Errorf("insert at list root requires a list value, got %v", val.WhichKind())
		}
		if err := appendDpbValues(c, val.GetL().GetValues(), fd, o.Types); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
		return nil

	case dpb.Entry_Test_case:
		val := entry.GetTest()
		if isClearValue(val) {
			if c.Len() != 0 {
				return fmt.Errorf("test failed at list root: list is not empty")
			}
			return nil
		}
		if val.WhichKind() != dpb.Value_L_case {
			return fmt.Errorf("test at list root requires a list value, got %v", val.WhichKind())
		}
		want := val.GetL().GetValues()
		if c.Len() != len(want) {
			return fmt.Errorf("test failed at list root: length %d != %d", c.Len(), len(want))
		}
		for i, ev := range want {
			expected, err := valueToProtoValue(ev, fd, o.Types)
			if err != nil {
				return fmt.Errorf("test: [%d]: %w", i, err)
			}
			if !protoValueEqual(c.Get(i), expected, fd.Kind()) {
				return fmt.Errorf("test failed at list root: index %d", i)
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported root op for list: %q", entry.WhichKind())
	}
}

// expandListTargets converts []*Segment to concrete index values.
func expandListTargets(targets []*dpb.Segment, l int) []int {
	out := make([]int, 0, len(targets))
	for _, seg := range targets {
		switch seg.WhichKind() {
		case dpb.Segment_Index_case:
			out = append(out, int(seg.GetIndex()))
		case dpb.Segment_Range_case:
			r := seg.GetRange()
			begin := int(r.GetBegin())
			end := int(r.GetEnd())
			if begin < 0 {
				begin = l + begin
			}
			if end <= 0 {
				end = l + end
			}
			for i := begin; i < end && i < l; i++ {
				out = append(out, i)
			}
		}
	}
	return out
}
