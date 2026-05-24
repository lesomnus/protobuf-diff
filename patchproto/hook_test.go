package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

// collectHook returns a WithHook option and a pointer to the collected paths.
func collectHook() (patchproto.Option, *[][]dpb.PathEntry) {
	var paths [][]dpb.PathEntry
	opt := patchproto.WithHook(func(p []dpb.PathEntry, _, _ dpb.Frame, _ *dpb.Entry) {
		cp := make([]dpb.PathEntry, len(p))
		copy(cp, p)
		paths = append(paths, cp)
	})
	return opt, &paths
}

// field creates a PathEntry for a protobuf message field with both name and field number.
func field(name string, number int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: name, Index: number}
}

// mapKey creates a PathEntry for a string map key (no field number).
func mapKey(key string) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: key}
}

func index(i int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i}
}

func TestHookMessage(t *testing.T) {
	t.Run("assigned notifies modified field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109)) // s_1
		d.SetAssigned(dpb.String("bar"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("s_1", 109)},
		}, *paths)
	})
	t.Run("assigned notifies each target field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		a.SetS_2("bar")

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109, 209)) // s_1, s_2
		d.SetAssigned(dpb.String("baz"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("s_1", 109)},
			{field("s_2", 209)},
		}, *paths)
	})
	t.Run("deleted notifies modified field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetOptS("foo")

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(409)) // opt_s
		d.SetDeleted(true)

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("opt_s", 409)},
		}, *paths)
	})
	t.Run("nested notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner := &sample.Value{}
		inner.SetS_1("foo")
		a.SetM_1(inner)

		// Patch a.m_1.s_1 = "bar"
		inner_d := &dpb.Entry{}
		inner_d.AppendTargets(target.Fields(109)) // s_1
		inner_d.SetAssigned(dpb.String("bar"))

		outer_d := &dpb.Entry{}
		outer_d.AppendTargets(target.Fields(111)) // m_1
		outer_d.SetNested(dpb.NewDelta(inner_d))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer_d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_1", 111), field("s_1", 109)},
		}, *paths)
	})
	t.Run("path navigation segments carry name only", func(t *testing.T) {
		a := &sample.Value{}
		inner := &sample.Value{}
		inner.SetS_1("foo")
		a.SetM_1(inner)

		// Navigation via path segments: only field name is available (no descriptor),
		// so Index stays 0. The final target "s_1" is resolved by patchMessage
		// which has the full FieldDescriptor, so it carries both name and number.
		d := &dpb.Entry{}
		d.SetPath(dpb.P.S("m_1", "s_1").Value())
		d.SetAssigned(dpb.String("bar"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_1", 0), field("s_1", 109)},
		}, *paths)
	})
	t.Run("no hook no error", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109))
		d.SetAssigned(dpb.String("bar"))

		_, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)
	})
	t.Run("no notification on error", func(t *testing.T) {
		a := &sample.Value{}

		// Field 999 does not exist.
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(999))
		d.SetAssigned(dpb.String("bar"))

		hook, paths := collectHook()
		patchproto.Patch(a, dpb.NewDelta(d), hook) //nolint:errcheck
		x.Eq(t, 0, len(*paths))
	})
}

func TestHookList(t *testing.T) {
	makeEntry := func(inner *dpb.Entry) *dpb.Entry {
		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(1009)) // r_s_1
		outer.SetNested(dpb.NewDelta(inner))
		return outer
	}

	t.Run("deleted notifies each removed index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0, 2))
		inner.SetDeleted(true)

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(makeEntry(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(0)},
			{field("r_s_1", 1009), index(2)},
		}, *paths)
	})
	t.Run("assigned update notifies each modified index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0, 2))
		inner.SetAssigned(dpb.String("z"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(makeEntry(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(0)},
			{field("r_s_1", 1009), index(2)},
		}, *paths)
	})
	t.Run("assigned insert notifies each insertion index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar"})

		inner := &dpb.Entry{}
		inner.SetNoUpdate(true)
		inner.AppendTargets(target.Indices(1)) // insert before index 1
		inner.SetAssigned(dpb.String("z"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(makeEntry(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(1)},
		}, *paths)
	})
	t.Run("swapped notifies both indices", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0))
		inner.SwappedWith(ref.Index(2))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(makeEntry(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(0)},
			{field("r_s_1", 1009), index(2)},
		}, *paths)
	})
	t.Run("nested notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner1 := &sample.Value{}
		inner1.SetS_1("foo")
		inner2 := &sample.Value{}
		inner2.SetS_1("bar")
		a.SetRM_1([]*sample.Value{inner1, inner2})

		// Patch r_m_1[1].s_1 = "baz"
		field_d := &dpb.Entry{}
		field_d.AppendTargets(target.Fields(109)) // s_1
		field_d.SetAssigned(dpb.String("baz"))

		list_d := &dpb.Entry{}
		list_d.AppendTargets(target.Indices(1))
		list_d.SetNested(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(1011)) // r_m_1
		outer.SetNested(dpb.NewDelta(list_d))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_m_1", 1011), index(1), field("s_1", 109)},
		}, *paths)
	})
}

func TestHookMap(t *testing.T) {
	t.Run("assigned notifies modified key", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"hello": "world"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("hello"))
		inner.SetAssigned(dpb.String("earth"))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(10909)) // m_s_s
		outer.SetNested(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("hello")},
		}, *paths)
	})
	t.Run("deleted notifies removed key", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"hello": "world", "foo": "bar"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("foo"))
		inner.SetDeleted(true)

		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(10909)) // m_s_s
		outer.SetNested(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("foo")},
		}, *paths)
	})
	t.Run("scattered notifies targets and source", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"src": "val", "dst": "old"})

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("dst"))
		inner.ScatteredFrom(ref.StringKey("src"))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(10909)) // m_s_s
		outer.SetNested(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		// "dst" is set, then "src" is cleared.
		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("dst")},
			{field("m_s_s", 10909), mapKey("src")},
		}, *paths)
	})
	t.Run("nested notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner := &sample.Value{}
		inner.SetS_1("foo")
		a.SetMSM(map[string]*sample.Value{"key": inner})

		field_d := &dpb.Entry{}
		field_d.AppendTargets(target.Fields(109)) // s_1
		field_d.SetAssigned(dpb.String("bar"))

		map_d := &dpb.Entry{}
		map_d.AppendTargets(target.StringKeys("key"))
		map_d.SetNested(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(10911)) // m_s_m
		outer.SetNested(dpb.NewDelta(map_d))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_m", 10911), mapKey("key"), field("s_1", 109)},
		}, *paths)
	})
}
