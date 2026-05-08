package patchproto

import (
	"fmt"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func (o PatchOption) patchList(c protoreflect.List, fd protoreflect.FieldDescriptor, entry *dpb.Entry) error {
	targets, err := target.DecodeIndices(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	l := c.Len()
	if l == 0 {
		if !entry.GetNoUpdate() {
			// Patch for update op on empty list is no-op.
			return nil
		}
	}

	normal := func(i int) (int, bool) {
		if i < 0 {
			i = l + i
		}
		if i < -l || i >= l {
			return 0, false
		}
		return i, true
	}
	splice := func(v protoreflect.Value) {
		insert_before := make([]int, 0, len(targets))
		pushbacks := 0
		for _, i := range targets {
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

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		targets_set := make(map[int]struct{}, len(targets))
		for _, i := range targets {
			i, ok := normal(i)
			if !ok {
				continue
			}

			targets_set[i] = struct{}{}
		}

		j := 0
		for i := range l {
			if _, ok := targets_set[i]; ok {
				continue
			}
			c.Set(j, c.Get(i))
			j++
		}
		c.Truncate(j)

	case dpb.Entry_Assigned_case:
		v, err := decodeValue(entry.GetAssigned(), fd)
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}

		if entry.GetNoUpdate() {
			splice(v)
		} else {
			for _, i := range targets {
				i, ok := normal(i)
				if !ok {
					continue
				}
				c.Set(i, v)
			}
		}

	case dpb.Entry_Copied_case:
		k, err := ref.DecodeIndex(entry.GetCopied())
		if err != nil {
			return fmt.Errorf("copy: unmarshal source index: %w", err)
		}

		l := c.Len()
		if !(-l <= k && k < l) {
			return nil
		}
		if k < 0 {
			k = l + k
		}

		v := c.Get(k)
		if entry.GetNoUpdate() {
			splice(v)
		} else {
			for _, i := range targets {
				i, ok := normal(i)
				if !ok {
					continue
				}
				c.Set(i, v)
			}
		}

	case dpb.Entry_Scattered_case:
		k, err := ref.DecodeIndex(entry.GetScattered())
		if err != nil {
			return fmt.Errorf("scatter: unmarshal source index: %w", err)
		}

		k, ok := normal(k)
		if !ok {
			return nil
		}

		v := c.Get(k)
		if entry.GetNoUpdate() {
			insert_before := make([]int, 0, len(targets))
			pushbacks := 0
			offset := 0
			for _, i := range targets {
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

			l = l + j + pushbacks
			for i := k + offset; i < l-1; i++ {
				c.Set(i, c.Get(i+1))
			}
		} else {
			for _, i := range targets {
				i, ok := normal(i)
				if !ok {
					continue
				}
				c.Set(i, v)
			}
			for i := k; i < l-1; i++ {
				c.Set(i, c.Get(i+1))
			}
		}
		c.Truncate(l - 1)

	case dpb.Entry_Swapped_case:
		k, err := ref.DecodeIndex(entry.GetSwapped())
		if err != nil {
			return fmt.Errorf("swap: unmarshal source index: %w", err)
		}

		k, ok := normal(k)
		if !ok {
			return nil
		}

		v := c.Get(k)
		for _, i := range targets {
			i, ok := normal(i)
			if !ok {
				continue
			}

			w := c.Get(i)
			c.Set(i, v)
			v = w
		}
		c.Set(k, v)

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()

		kind := fd.Kind()
		if !(kind == protoreflect.MessageKind || kind == protoreflect.GroupKind) {
			return fmt.Errorf("nested deltas for lists can only be applied to messages, got %q", kind.String())
		}

		if entry.GetNoUpdate() {
			mt, err := protoregistry.GlobalTypes.FindMessageByName(fd.Message().FullName())
			if err != nil {
				return fmt.Errorf("find message type %q: %w", fd.Message().FullName(), err)
			}

			sub := mt.New()
			if err := o.PatchField(sub, fd, delta); err != nil {
				return err
			}
			splice(protoreflect.ValueOfMessage(sub))
		} else {
			for _, i := range targets {
				i, ok := normal(i)
				if !ok {
					continue
				}

				v := c.Get(i)
				sub := v.Message()
				if err := o.PatchField(sub, fd, delta); err != nil {
					return fmt.Errorf("[%d]: %w", i, err)
				}
				c.Set(i, protoreflect.ValueOfMessage(sub))
			}
		}
	}

	return nil
}
