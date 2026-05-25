package patchjson_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchjson"
)

func collectJsonHook() (patchjson.Option, *[][]dpb.PathEntry) {
	var paths [][]dpb.PathEntry
	opt := patchjson.WithHook(func(p []dpb.PathEntry, _, _ dpb.Frame, _ *dpb.Entry) {
		cp := make([]dpb.PathEntry, len(p))
		copy(cp, p)
		paths = append(paths, cp)
	})
	return opt, &paths
}

func jField(name string) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: name}
}

func jIndex(i int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: i}
}

func TestHookJsonMap(t *testing.T) {
	t.Run("assign notifies modified key", func(t *testing.T) {
		m := map[string]any{"a": "old"}

		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		e.SetAssign(dpb.ValS("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
		}, *paths)
	})
	t.Run("assign notifies each target key", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}

		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a"), dpb.SegName("b")})
		e.SetAssign(dpb.ValS("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
			{jField("b")},
		}, *paths)
	})
	t.Run("remove notifies removed key", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}

		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		e.SetRemove(true)

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
		}, *paths)
	})
	t.Run("move notifies target and source", func(t *testing.T) {
		m := map[string]any{"src": "val", "dst": "old"}

		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("dst")})
		e.SetMove(dpb.Field("src"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("dst")},
			{jField("src")},
		}, *paths)
	})
	t.Run("nest notifies with full path", func(t *testing.T) {
		m := map[string]any{
			"outer": map[string]any{"inner": "old"},
		}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("inner")})
		inner.SetAssign(dpb.ValS("new"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("outer")})
		outer.SetNest(dpb.NewDelta(inner))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(outer), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("outer"), jField("inner")},
		}, *paths)
	})
	t.Run("path navigation segments included", func(t *testing.T) {
		m := map[string]any{
			"outer": map[string]any{"inner": "old"},
		}

		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("outer")))
		e.SetTargets([]*dpb.Segment{dpb.SegName("inner")})
		e.SetAssign(dpb.ValS("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("outer"), jField("inner")},
		}, *paths)
	})
	t.Run("no hook no error", func(t *testing.T) {
		m := map[string]any{"a": "old"}

		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		e.SetAssign(dpb.ValS("new"))

		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e)))
	})
}

func TestHookJsonList(t *testing.T) {
	makeEntry := func(inner *dpb.Entry) *dpb.Entry {
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("list")})
		outer.SetNest(dpb.NewDelta(inner))
		return outer
	}

	t.Run("remove notifies each removed index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		inner.SetRemove(true)

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("assign notifies each modified index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		inner.SetAssign(dpb.ValS("z"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("insert notifies each insertion index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b"}}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetInsert(dpb.ValS("z"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(1)},
		}, *paths)
	})
	t.Run("move notifies insert and source removal", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(0)})
		inner.SetMove(dpb.FieldNum(2))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("nest notifies with full path", func(t *testing.T) {
		m := map[string]any{
			"list": []any{
				map[string]any{"v": "old"},
				map[string]any{"v": "old"},
			},
		}

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("v")})
		inner.SetAssign(dpb.ValS("new"))

		list_e := &dpb.Entry{}
		list_e.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		list_e.SetNest(dpb.NewDelta(inner))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("list")})
		outer.SetNest(dpb.NewDelta(list_e))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(outer), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(1), jField("v")},
		}, *paths)
	})
}
