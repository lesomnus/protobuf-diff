package patchjson_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchjson"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func collectJsonHook() (patchjson.Option, *[][]dpb.PathEntry) {
	var paths [][]dpb.PathEntry
	opt := patchjson.WithHook(func(p []dpb.PathEntry, _ *dpb.Entry) {
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
	t.Run("assigned notifies modified key", func(t *testing.T) {
		m := map[string]any{"a": "old"}

		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SetAssigned(patchjson.Value("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
		}, *paths)
	})
	t.Run("assigned notifies each target key", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}

		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a", "b"))
		e.SetAssigned(patchjson.Value("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
			{jField("b")},
		}, *paths)
	})
	t.Run("deleted notifies removed key", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}

		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SetDeleted(true)

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("a")},
		}, *paths)
	})
	t.Run("scattered notifies targets and source", func(t *testing.T) {
		m := map[string]any{"src": "val", "dst": "old"}

		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("dst"))
		e.ScatteredFrom(ref.StringKey("src"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("dst")},
			{jField("src")},
		}, *paths)
	})
	t.Run("nested notifies with full path", func(t *testing.T) {
		m := map[string]any{
			"outer": map[string]any{"inner": "old"},
		}

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("inner"))
		inner.SetAssigned(patchjson.Value("new"))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("outer"))
		outer.SetNested(dpb.NewDelta(inner))

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
		e.SetPath(dpb.P.S("outer", "inner").Value())
		e.SetAssigned(patchjson.Value("new"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("outer"), jField("inner")},
		}, *paths)
	})
	t.Run("no hook no error", func(t *testing.T) {
		m := map[string]any{"a": "old"}

		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SetAssigned(patchjson.Value("new"))

		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(e)))
	})
}

func TestHookJsonList(t *testing.T) {
	makeEntry := func(inner *dpb.Entry) *dpb.Entry {
		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("list"))
		outer.SetNested(dpb.NewDelta(inner))
		return outer
	}

	t.Run("deleted notifies each removed index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0, 2))
		inner.SetDeleted(true)

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("assigned update notifies each modified index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0, 2))
		inner.SetAssigned(patchjson.Value("z"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("assigned insert notifies each insertion index", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b"}}

		inner := &dpb.Entry{}
		inner.SetNoUpdate(true)
		inner.AppendTargets(target.Indices(1))
		inner.SetAssigned(patchjson.Value("z"))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(1)},
		}, *paths)
	})
	t.Run("swapped notifies both indices", func(t *testing.T) {
		m := map[string]any{"list": []any{"a", "b", "c"}}

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0))
		inner.SwappedWith(ref.Index(2))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(makeEntry(inner)), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(0)},
			{jField("list"), jIndex(2)},
		}, *paths)
	})
	t.Run("nested notifies with full path", func(t *testing.T) {
		m := map[string]any{
			"list": []any{
				map[string]any{"v": "old"},
				map[string]any{"v": "old"},
			},
		}

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("v"))
		inner.SetAssigned(patchjson.Value("new"))

		list_e := &dpb.Entry{}
		list_e.AppendTargets(target.Indices(1))
		list_e.SetNested(dpb.NewDelta(inner))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("list"))
		outer.SetNested(dpb.NewDelta(list_e))

		hook, paths := collectJsonHook()
		x.NoError(t, patchjson.Patch(m, dpb.NewDelta(outer), hook))

		x.Eq(t, [][]dpb.PathEntry{
			{jField("list"), jIndex(1), jField("v")},
		}, *paths)
	})
}
