package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func TestNavigate(t *testing.T) {
	t.Run("unsupported container type returns error", func(t *testing.T) {
		_, _, err := patchproto.Navigate("not a proto container", nil, []any{"field"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("message", func(t *testing.T) {
		new_msg := func() protoreflect.Message {
			return (&sample.Value{}).ProtoReflect()
		}

		t.Run("empty segments returns message", func(t *testing.T) {
			m := new_msg()
			got, fd, err := patchproto.Navigate(m, nil, nil)
			x.NoError(t, err)
			x.Same(t, m, got)
			x.Eq(t, fd, nil)
		})
		t.Run("by field name", func(t *testing.T) {
			_, fd, err := patchproto.Navigate(new_msg(), nil, []any{"s_1"})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("by int field number", func(t *testing.T) {
			_, fd, err := patchproto.Navigate(new_msg(), nil, []any{109})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("by uint field number", func(t *testing.T) {
			_, fd, err := patchproto.Navigate(new_msg(), nil, []any{uint(109)})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("unknown field name returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{"nonexistent"})
			x.Error(t, err)
		})
		t.Run("field number 0 returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{0})
			x.Error(t, err)
		})
		t.Run("negative field number returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{-1})
			x.Error(t, err)
		})
		t.Run("uint field number 0 returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{uint(0)})
			x.Error(t, err)
		})
		t.Run("unknown uint field number returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{uint(99999)})
			x.Error(t, err)
		})
		t.Run("invalid segment type returns error", func(t *testing.T) {
			_, _, err := patchproto.Navigate(new_msg(), nil, []any{3.14})
			x.Error(t, err)
		})
	})
	t.Run("list", func(t *testing.T) {
		fields := (&sample.Value{}).ProtoReflect().Descriptor().Fields()
		// r_m_1 is repeated Value (message list), field number 1011.
		newMsgList := func() (protoreflect.List, protoreflect.FieldDescriptor) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("r_m_1")
			list := m.Mutable(fd).List()
			list.AppendMutable()
			return list, fd
		}
		t.Run("empty segments returns list", func(t *testing.T) {
			list, fd := newMsgList()
			got, god_fd, err := patchproto.Navigate(list, fd, nil)
			x.NoError(t, err)

			x.TypeImpl(t, (*protoreflect.List)(nil), got)
			x.Same(t, list, got)
			x.Eq(t, fd, god_fd)
		})
		t.Run("positive int index returns message", func(t *testing.T) {
			list, fd := newMsgList()
			got, _, err := patchproto.Navigate(list, fd, []any{0})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
		t.Run("negative int index wraps from end", func(t *testing.T) {
			list, fd := newMsgList()
			got, _, err := patchproto.Navigate(list, fd, []any{-1})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
		t.Run("uint index returns message", func(t *testing.T) {
			list, fd := newMsgList()
			got, _, err := patchproto.Navigate(list, fd, []any{uint(0)})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
		t.Run("int index out of bounds returns error", func(t *testing.T) {
			list, fd := newMsgList() // length = 1
			_, _, err := patchproto.Navigate(list, fd, []any{2})
			x.Error(t, err)
		})
		t.Run("uint index out of bounds returns error", func(t *testing.T) {
			list, fd := newMsgList() // length = 1
			_, _, err := patchproto.Navigate(list, fd, []any{uint(2)})
			x.Error(t, err)
		})
		t.Run("negative int index out of bounds returns error", func(t *testing.T) {
			list, fd := newMsgList() // length = 1
			_, _, err := patchproto.Navigate(list, fd, []any{-2})
			x.Error(t, err)
		})
		t.Run("string segment returns error", func(t *testing.T) {
			list, fd := newMsgList()
			_, _, err := patchproto.Navigate(list, fd, []any{"not_an_index"})
			x.Error(t, err)
		})
		// r_s_1 is repeated string (scalar list), field number 1009.
		t.Run("scalar element cannot be navigated into", func(t *testing.T) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("r_s_1")
			list := m.Mutable(fd).List()
			list.Append(protoreflect.ValueOfString("foo"))
			_, _, err := patchproto.Navigate(list, fd, []any{0})
			x.Error(t, err)
		})
	})
	t.Run("map", func(t *testing.T) {
		fields := (&sample.Value{}).ProtoReflect().Descriptor().Fields()
		t.Run("empty segments returns map", func(t *testing.T) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("m_s_m")
			mp := m.Mutable(fd).Map()

			got, gotFd, err := patchproto.Navigate(mp, fd, nil)
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Map)(nil), got)
			x.Same(t, mp, got)
			x.Eq(t, fd, gotFd)
		})
		t.Run("string key navigates to message value", func(t *testing.T) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("m_s_m")
			mp := m.Mutable(fd).Map()
			mp.Set(protoreflect.ValueOfString("key").MapKey(), mp.NewValue())

			got, _, err := patchproto.Navigate(mp, fd, []any{"key"})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
		t.Run("int key is coerced to string key", func(t *testing.T) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("m_s_m")
			mp := m.Mutable(fd).Map()
			mp.Set(protoreflect.ValueOfString("42").MapKey(), mp.NewValue())

			got, _, err := patchproto.Navigate(mp, fd, []any{42})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
		t.Run("uint key is coerced to string key", func(t *testing.T) {
			m := (&sample.Value{}).ProtoReflect()
			fd := fields.ByName("m_s_m")
			mp := m.Mutable(fd).Map()
			mp.Set(protoreflect.ValueOfString("42").MapKey(), mp.NewValue())

			got, _, err := patchproto.Navigate(mp, fd, []any{uint(42)})
			x.NoError(t, err)
			x.TypeImpl(t, (*protoreflect.Message)(nil), got)
		})
	})
	t.Run("nested", func(t *testing.T) {
		t.Run("message to message field", func(t *testing.T) {
			sub := &sample.Value{}
			sub.SetS_1("hello")
			msg := &sample.Value{}
			msg.SetM_1(sub)

			_, fd, err := patchproto.Navigate(msg.ProtoReflect(), nil, []any{"m_1", "s_1"})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("message to list element field", func(t *testing.T) {
			sub := &sample.Value{}
			sub.SetS_1("hello")
			msg := &sample.Value{}
			msg.SetRM_1([]*sample.Value{sub})

			_, fd, err := patchproto.Navigate(msg.ProtoReflect(), nil, []any{"r_m_1", 0, "s_1"})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("message to map value field", func(t *testing.T) {
			sub := &sample.Value{}
			sub.SetS_1("hello")
			msg := &sample.Value{}
			msg.SetMSM(map[string]*sample.Value{"key": sub})

			_, fd, err := patchproto.Navigate(msg.ProtoReflect(), nil, []any{"m_s_m", "key", "s_1"})
			x.NoError(t, err)
			x.Eq(t, protoreflect.Name("s_1"), fd.Name())
		})
		t.Run("into scalar field returns error", func(t *testing.T) {
			msg := &sample.Value{}
			msg.SetS_1("hello")

			_, _, err := patchproto.Navigate(msg.ProtoReflect(), nil, []any{"s_1", "nonexistent"})
			x.Error(t, err)
		})
	})
}
