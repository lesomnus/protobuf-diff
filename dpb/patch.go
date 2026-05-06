package dpb

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Patcher interface {
	Patch(v proto.Message, delta *Delta) error
}

type PatchOption struct{}

func Patch(v proto.Message, delta *Delta) error {
	return PatchOption{}.Patch(v, delta)
}

func Patched[T proto.Message](v T, delta *Delta) (T, error) {
	v = proto.CloneOf(v)
	if err := Patch(v, delta); err != nil {
		var z T
		return z, err
	}
	return v, nil
}

func (o PatchOption) Patch(v proto.Message, delta *Delta) error {
	return o.applyDeltaMessage(v.ProtoReflect(), delta)
}

func (o PatchOption) applyDeltaMessage(c protoreflect.Message, delta *Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.applyEntryMessage(c, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (o PatchOption) applyEntryMessage(c protoreflect.Message, entry *Entry) error {
	targets, err := target.DecodeFieldNumbers(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	fields := c.Descriptor().Fields()
	fd0 := fields.ByNumber(targets[0])
	if fd0 == nil {
		return fmt.Errorf("field [%d] not found in %q", targets[0], c.Descriptor().FullName())
	}

	op := func(fd protoreflect.FieldDescriptor) error {
		return nil
	}
	check := func(fd protoreflect.FieldDescriptor) bool {
		exists := true
		if fd.HasPresence() {
			exists = c.Has(fd)
		}

		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	switch entry.WhichKind() {
	case Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(fd protoreflect.FieldDescriptor) error {
				c.Clear(fd)
				return nil
			}
		}

	case Entry_Assigned_case:
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}

			v, err := decodeValue(entry.GetAssigned(), fd)
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}

			c.Set(fd, v)
			return nil
		}

	case Entry_Merged_case:
		var v proto.Message
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}
			if fd.Kind() != protoreflect.MessageKind {
				return fmt.Errorf("field must be message type %q", fd.FullName())
			}
			if v == nil {
				m := dynamicpb.NewMessageType(fd.Message()).New()
				v = m.Interface()
				if err := proto.Unmarshal(entry.GetMerged(), v); err != nil {
					return fmt.Errorf("unmarshal: %w", err)
				}
			}

			w := c.Get(fd)
			proto.Merge(w.Message().Interface(), v)
			return nil
		}

	case Entry_Copied_case:
		k, err := ref.DecodeField(entry.GetCopied())
		if err != nil {
			return fmt.Errorf("copy: unmarshal source field number: %w", err)
		}

		fd_src := fields.ByNumber(k)
		if !c.Has(fd_src) {
			// Source field is not set, so clear target fields without setting.
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				c.Clear(fd)
				return nil
			}
		} else {
			v := c.Get(fd_src)
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				w, err := o.cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}

				c.Set(fd, w)
				return nil
			}
		}

	case Entry_Scattered_case:
		k, err := ref.DecodeField(entry.GetScattered())
		if err != nil {
			return fmt.Errorf("scatter: unmarshal source field number: %w", err)
		}

		fd_src := fields.ByNumber(k)
		if !c.Has(fd_src) {
			// Source field is not set, so clear target fields without setting.
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				c.Clear(fd)
				return nil
			}
		} else {
			v := c.Get(fd_src)
			done := false
			op = func(fd protoreflect.FieldDescriptor) error {
				if !check(fd) {
					return nil
				}

				w, err := o.cast(v, fd_src.Kind(), fd.Kind())
				if err != nil {
					return err
				}

				c.Set(fd, w)
				if !done {
					c.Clear(fd_src)
					done = true
				}
				return nil
			}
		}

	case Entry_Swapped_case:
		target, err := ref.DecodeField(entry.GetSwapped())
		if err != nil {
			return fmt.Errorf("swap: unmarshal target field number: %w", err)
		}

		op = func(fd_src protoreflect.FieldDescriptor) error {
			fd_dst := fields.ByNumber(target)
			if fd_dst == nil {
				return nil
			}

			ka := fd_src.Kind()
			kb := fd_dst.Kind()

			va, err := o.cast(c.Get(fd_src), ka, kb)
			if err != nil {
				return fmt.Errorf("cast src: %w", err)
			}

			vb, err := o.cast(c.Get(fd_dst), kb, ka)
			if err != nil {
				return fmt.Errorf("cast dst: %w", err)
			}

			c.Set(fd_src, vb)
			c.Set(fd_dst, va)
			return nil
		}

	case Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	case Entry_Nested_case:
		delta := entry.GetNested()
		op = func(fd protoreflect.FieldDescriptor) error {
			if !check(fd) {
				return nil
			}

			kind := fd.Kind()
			switch {
			case fd.IsList():
				sub := c.Mutable(fd).List()
				if err := o.applyDeltaList(sub, fd, delta); err != nil {
					return fmt.Errorf("field %s: %w", fd.FullName(), err)
				}

			case fd.IsMap():
				sub := c.Mutable(fd).Map()
				if err := o.applyDeltaMap(sub, fd, delta); err != nil {
					return fmt.Errorf("field %s: %w", fd.FullName(), err)
				}

			case kind == protoreflect.MessageKind || kind == protoreflect.GroupKind:
				sub := c.Mutable(fd).Message()
				if err := o.applyDeltaMessage(sub, delta); err != nil {
					return fmt.Errorf("field %s: %w", fd.FullName(), err)
				}

			default:
				return fmt.Errorf("field %s: nested delta cannot be applied to %q", fd.FullName(), kind.String())
			}
			return nil
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(targets))
	for _, i := range targets {
		fd := fields.ByNumber(i)
		if fd == nil {
			continue
		}

		if err := op(fd); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func (o PatchOption) applyDeltaList(c protoreflect.List, fd protoreflect.FieldDescriptor, delta *Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.applyEntryList(c, fd, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (o PatchOption) applyEntryList(c protoreflect.List, fd protoreflect.FieldDescriptor, entry *Entry) error {
	targets, err := target.DecodeIndices(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	l := c.Len()
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
	case Entry_Deleted_case:
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

	case Entry_Assigned_case:
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

	case Entry_Copied_case:
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

	case Entry_Scattered_case:
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

	case Entry_Swapped_case:
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

	case Entry_Nested_case:
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

			m := mt.New()
			if err := o.applyDeltaMessage(m, delta); err != nil {
				return err
			}
			splice(protoreflect.ValueOfMessage(m))
		} else {
			for _, i := range targets {
				i, ok := normal(i)
				if !ok {
					continue
				}

				v := c.Get(i)
				sub := v.Message()
				if err := o.applyDeltaMessage(sub, delta); err != nil {
					return fmt.Errorf("[%d]: %w", i, err)
				}
				c.Set(i, protoreflect.ValueOfMessage(sub))
			}
		}
	}

	return nil
}

func (o PatchOption) applyDeltaMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, delta *Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.applyEntryMap(c, fd, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (o PatchOption) applyEntryMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, entry *Entry) error {
	kd := fd.MapKey()
	keys, err := target.DecodeKeys(entry.GetTargets(), kd.Kind())
	if err != nil {
		return fmt.Errorf("decode keys: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	vd := fd.MapValue()
	op := func(k protoreflect.MapKey) error { return nil }
	after := func() error { return nil }
	check := func(k protoreflect.MapKey) bool {
		exists := c.Has(k)
		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	switch entry.WhichKind() {
	case Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(k protoreflect.MapKey) error {
				c.Clear(k)
				return nil
			}
		}

	case Entry_Assigned_case:
		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if !check(k) {
				return nil
			}
			if !v.IsValid() {
				var err error
				v, err = decodeValue(entry.GetAssigned(), vd)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
			}

			c.Set(k, v)
			return nil
		}

	case Entry_Merged_case:
		if vd.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("value must be message type %q", vd.FullName())
		}

		var v protoreflect.Value
		op = func(k protoreflect.MapKey) error {
			if !check(k) {
				return nil
			}
			if !v.IsValid() {
				var err error
				v, err = decodeValue(entry.GetAssigned(), vd)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
			}

			w := c.Get(k)
			proto.Merge(w.Message().Interface(), v.Message().Interface())
			return nil
		}

	case Entry_Copied_case:
		src, err := ref.DecodeKey(entry.GetCopied(), kd.Kind())
		if err != nil {
			return fmt.Errorf("copy: unmarshal source key: %w", err)
		}
		if !c.Has(src) {
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				if !v.IsValid() {
					v = c.Get(src)
				}
				c.Set(k, v)
				return nil
			}
		}

	case Entry_Scattered_case:
		src, err := ref.DecodeKey(entry.GetScattered(), kd.Kind())
		if err != nil {
			return fmt.Errorf("scatter: unmarshal source key: %w", err)
		}
		if !c.Has(src) {
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				c.Clear(k)
				return nil
			}
		} else {
			var v protoreflect.Value
			op = func(k protoreflect.MapKey) error {
				if !check(k) {
					return nil
				}

				if !v.IsValid() {
					v = c.Get(src)
				}
				c.Set(k, v)
				return nil
			}
		}
		after = func() error {
			c.Clear(src)
			return nil
		}

	case Entry_Swapped_case:
		src, err := ref.DecodeKey(entry.GetSwapped(), kd.Kind())
		if err != nil {
			return fmt.Errorf("swap: unmarshal source key: %w", err)
		}

		v := c.Get(src)
		op = func(k protoreflect.MapKey) error {
			w := c.Get(k)
			c.Set(k, v)
			v = w
			return nil
		}
		after = func() error {
			c.Set(src, v)
			return nil
		}

	case Entry_Nested_case:
		delta := entry.GetNested()
		kind := vd.Kind()
		if !(kind == protoreflect.MessageKind || kind == protoreflect.GroupKind) {
			return fmt.Errorf("nested deltas for maps can only be applied to message values, got %q", kind.String())
		}

		op = func(k protoreflect.MapKey) error {
			if !c.Has(k) {
				return nil
			}

			v := c.Get(k)
			sub := v.Message()
			if err := o.applyDeltaMessage(sub, delta); err != nil {
				return fmt.Errorf("key %v: %w", k, err)
			}
			c.Set(k, protoreflect.ValueOfMessage(sub))
			return nil
		}

	case Entry_Edited_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, c.Len())
	for _, k := range keys {
		if err := op(k); err != nil {
			errs = append(errs, fmt.Errorf("[%v]: %w", k, err))
		}
	}
	if err := after(); err != nil {
		errs = append(errs, fmt.Errorf("clean up: %w", err))
	}
	return errors.Join(errs...)
}

// decodeValue decodes a field value from its wire-format bytes based on the field descriptor.
func decodeValue(b []byte, fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfBool(v != 0), nil

	case protoreflect.EnumKind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfEnum(protoreflect.EnumNumber(int32(v))), nil

	case protoreflect.Int32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt32(int32(v)), nil

	case protoreflect.Sint32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt32(int32(protowire.DecodeZigZag(v))), nil

	case protoreflect.Uint32Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfUint32(uint32(v)), nil

	case protoreflect.Int64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt64(int64(v)), nil

	case protoreflect.Sint64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfInt64(protowire.DecodeZigZag(v)), nil

	case protoreflect.Uint64Kind:
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid varint")
		}
		return protoreflect.ValueOfUint64(v), nil

	case protoreflect.Sfixed32Kind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfInt32(int32(v)), nil

	case protoreflect.Fixed32Kind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfUint32(v), nil

	case protoreflect.FloatKind:
		v, n := protowire.ConsumeFixed32(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed32")
		}
		return protoreflect.ValueOfFloat32(math.Float32frombits(v)), nil

	case protoreflect.Sfixed64Kind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfInt64(int64(v)), nil

	case protoreflect.Fixed64Kind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfUint64(v), nil

	case protoreflect.DoubleKind:
		v, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return protoreflect.Value{}, errors.New("invalid fixed64")
		}
		return protoreflect.ValueOfFloat64(math.Float64frombits(v)), nil

	case protoreflect.StringKind:
		return protoreflect.ValueOfString(string(b)), nil

	case protoreflect.BytesKind:
		cp := make([]byte, len(b))
		copy(cp, b)
		return protoreflect.ValueOfBytes(cp), nil

	case protoreflect.MessageKind, protoreflect.GroupKind:
		mt, err := protoregistry.GlobalTypes.FindMessageByName(fd.Message().FullName())
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("find message type %q: %w", fd.Message().FullName(), err)
		}

		m := mt.New()
		if err := proto.Unmarshal(b, m.Interface()); err != nil {
			return protoreflect.Value{}, fmt.Errorf("unmarshal message: %w", err)
		}

		return protoreflect.ValueOfMessage(m), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}

func (o PatchOption) cast(v protoreflect.Value, from, to protoreflect.Kind) (protoreflect.Value, error) {
	if from == to {
		return v, nil
	}

	w, ok := cast(v, from, to)
	if !ok {
		return protoreflect.Value{}, ErrInvalidCast{from, to}
	}
	return w, nil
}
