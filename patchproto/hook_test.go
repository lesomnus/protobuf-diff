package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

func collectHook() (patchproto.Option, *[][]dpb.PathEntry) {
	var paths [][]dpb.PathEntry
	opt := patchproto.WithHook(func(p []dpb.PathEntry, _, _ dpb.Frame, _ *dpb.Entry) {
		cp := make([]dpb.PathEntry, len(p))
		copy(cp, p)
		paths = append(paths, cp)
	})
	return opt, &paths
}

func field(name string, number int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: name, Index: number}
}

func mapKey(key string) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: key}
}

func index(i int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i}
}

func TestHookMessage(t *testing.T) {
	t.Run("assign notifies modified field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetAssign(dpb.ValS("bar"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("s_1", 109)},
		}, *paths)
	})
	t.Run("assign notifies each target field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		a.SetS_2("bar")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{
			dpb.SegField(dpb.FieldNum(109)), // s_1
			dpb.SegField(dpb.FieldNum(209)), // s_2
		})
		d.SetAssign(dpb.ValS("baz"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("s_1", 109)},
			{field("s_2", 209)},
		}, *paths)
	})
	t.Run("remove notifies modified field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetOptS("foo")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(409))}) // opt_s
		d.SetRemove(true)

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("opt_s", 409)},
		}, *paths)
	})
	t.Run("nest notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner := &sample.Value{}
		inner.SetS_1("foo")
		a.SetM_1(inner)

		sub := &dpb.Entry{}
		sub.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		sub.SetAssign(dpb.ValS("bar"))

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(111))}) // m_1
		d.SetNest(dpb.NewDelta(sub))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
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

		// Navigate via path (name only → Index stays 0), target within m_1
		d := &dpb.Entry{}
		d.SetPath(dpb.PathOf(dpb.Field("m_1")))
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(209))}) // s_2
		d.SetAssign(dpb.ValS("bar"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_1", 0), field("s_2", 209)},
		}, *paths)
	})
	t.Run("no hook no error", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))})
		d.SetAssign(dpb.ValS("bar"))

		_, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)
	})
	t.Run("unknown field no notification", func(t *testing.T) {
		a := &sample.Value{}

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(9999))})
		d.SetAssign(dpb.ValS("bar"))

		hook, paths := collectHook()
		patchproto.Patch(a, dpb.NewDelta(d), hook) //nolint:errcheck
		x.Eq(t, 0, len(*paths))
	})
}

func TestHookList(t *testing.T) {
	wrapList := func(inner *dpb.Entry) *dpb.Entry {
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1009))}) // r_s_1
		outer.SetNest(dpb.NewDelta(inner))
		return outer
	}

	t.Run("remove notifies each removed index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		inner.SetRemove(true)

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapList(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(0)},
			{field("r_s_1", 1009), index(2)},
		}, *paths)
	})
	t.Run("assign notifies each modified index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		inner.SetAssign(dpb.ValS("z"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapList(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(0)},
			{field("r_s_1", 1009), index(2)},
		}, *paths)
	})
	t.Run("insert notifies each insertion index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)}) // insert before index 1
		inner.SetInsert(dpb.ValS("z"))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapList(inner)), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_s_1", 1009), index(1)},
		}, *paths)
	})
	t.Run("nest notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner1 := &sample.Value{}
		inner1.SetS_1("foo")
		inner2 := &sample.Value{}
		inner2.SetS_1("bar")
		a.SetRM_1([]*sample.Value{inner1, inner2})

		field_d := &dpb.Entry{}
		field_d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		field_d.SetAssign(dpb.ValS("baz"))

		list_d := &dpb.Entry{}
		list_d.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		list_d.SetNest(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1011))}) // r_m_1
		outer.SetNest(dpb.NewDelta(list_d))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("r_m_1", 1011), index(1), field("s_1", 109)},
		}, *paths)
	})
}

func TestHookMap(t *testing.T) {
	t.Run("assign notifies modified key", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"hello": "world"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("hello")})
		inner.SetAssign(dpb.ValS("earth"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10909))}) // m_s_s
		outer.SetNest(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("hello")},
		}, *paths)
	})
	t.Run("remove notifies removed key", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"hello": "world", "foo": "bar"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("foo")})
		inner.SetRemove(true)

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10909))}) // m_s_s
		outer.SetNest(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("foo")},
		}, *paths)
	})
	t.Run("move notifies target and source", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"src": "val", "dst": "old"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("dst")})
		inner.SetMove(dpb.Field("src"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10909))}) // m_s_s
		outer.SetNest(dpb.NewDelta(inner))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		// "dst" is set (move write), then "src" is cleared (move cleanup)
		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_s", 10909), mapKey("dst")},
			{field("m_s_s", 10909), mapKey("src")},
		}, *paths)
	})
	t.Run("nest notifies with full path", func(t *testing.T) {
		a := &sample.Value{}
		inner := &sample.Value{}
		inner.SetS_1("foo")
		a.SetMSM(map[string]*sample.Value{"key": inner})

		field_d := &dpb.Entry{}
		field_d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		field_d.SetAssign(dpb.ValS("bar"))

		map_d := &dpb.Entry{}
		map_d.SetTargets([]*dpb.Segment{dpb.SegName("key")})
		map_d.SetNest(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10911))}) // m_s_m
		outer.SetNest(dpb.NewDelta(map_d))

		hook, paths := collectHook()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, [][]dpb.PathEntry{
			{field("m_s_m", 10911), mapKey("key"), field("s_1", 109)},
		}, *paths)
	})
}
