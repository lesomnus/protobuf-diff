package patchjson_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchjson"
)

func mustPatch(t *testing.T, v any, delta *dpb.Delta) {
	t.Helper()
	if err := patchjson.Patch(v, delta); err != nil {
		t.Fatalf("Patch: %v", err)
	}
}

func TestPatchMap(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		e.SetRemove(true)
		mustPatch(t, m, dpb.NewDelta(e))
		if _, ok := m["a"]; ok {
			t.Fatal("expected key 'a' removed")
		}
		x.Eq(t, "2", m["b"])
	})
	t.Run("assign", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a"), dpb.SegName("b")})
		e.SetAssign(dpb.ValS("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "new", m["a"])
		x.Eq(t, "new", m["b"])
	})
	t.Run("assign null", func(t *testing.T) {
		m := map[string]any{"a": "v"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		e.SetAssign(dpb.ValNull())
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, nil, m["a"])
	})
	t.Run("assign number", func(t *testing.T) {
		m := map[string]any{}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("n")})
		e.SetAssign(dpb.ValF(3.14))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, 3.14, m["n"])
	})
	t.Run("assign bool", func(t *testing.T) {
		m := map[string]any{}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("ok")})
		e.SetAssign(dpb.ValB(true))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, true, m["ok"])
	})
	t.Run("insert absent key", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("a"), dpb.SegName("b")})
		e.SetInsert(dpb.ValS("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "old", m["a"]) // existing key unchanged
		x.Eq(t, "new", m["b"]) // absent key inserted
	})
	t.Run("copy", func(t *testing.T) {
		m := map[string]any{"src": "hello", "dst": "old"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("dst")})
		e.SetCopy(dpb.Field("src"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "hello", m["src"])
		x.Eq(t, "hello", m["dst"])
	})
	t.Run("move", func(t *testing.T) {
		m := map[string]any{"src": "hello", "dst": "old"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("dst")})
		e.SetMove(dpb.Field("src"))
		mustPatch(t, m, dpb.NewDelta(e))
		if _, ok := m["src"]; ok {
			t.Fatal("expected src removed after move")
		}
		x.Eq(t, "hello", m["dst"])
	})
	t.Run("nest", func(t *testing.T) {
		m := map[string]any{"obj": map[string]any{"x": "old"}}
		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("x")})
		inner.SetAssign(dpb.ValS("new"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("obj")})
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		x.Eq(t, "new", m["obj"].(map[string]any)["x"])
	})
	t.Run("path-based navigation", func(t *testing.T) {
		m := map[string]any{"outer": map[string]any{"inner": "old"}}
		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("outer")))
		e.SetTargets([]*dpb.Segment{dpb.SegName("inner")})
		e.SetAssign(dpb.ValS("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "new", m["outer"].(map[string]any)["inner"])
	})
}

func TestPatchList(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		e.SetRemove(true)
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 2)
		x.Eq(t, "a", (*sl)[0])
		x.Eq(t, "c", (*sl)[1])
	})
	t.Run("assign", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		e.SetAssign(dpb.ValS("z"))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Eq(t, "z", (*sl)[1])
	})
	t.Run("insert appends", func(t *testing.T) {
		sl := &[]any{"a", "b"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegIndex(-1)})
		e.SetInsert(dpb.ValS("c"))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 3)
		x.Eq(t, "c", (*sl)[2])
	})
	t.Run("copy splices element", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		e.SetCopy(dpb.FieldNum(2))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 4)
		x.Eq(t, "a", (*sl)[0])
		x.Eq(t, "c", (*sl)[1])
		x.Eq(t, "b", (*sl)[2])
		x.Eq(t, "c", (*sl)[3])
	})
	t.Run("move to end", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegIndex(-1)})
		e.SetMove(dpb.FieldNum(0))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 3)
		x.Eq(t, "b", (*sl)[0])
		x.Eq(t, "c", (*sl)[1])
		x.Eq(t, "a", (*sl)[2])
	})
	t.Run("nest list-in-map", func(t *testing.T) {
		m := map[string]any{"arr": []any{"a", "b", "c"}}
		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetAssign(dpb.ValS("z"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("arr")})
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		x.Eq(t, "z", m["arr"].([]any)[1])
	})
	t.Run("nest remove in list-in-map", func(t *testing.T) {
		m := map[string]any{"arr": []any{"a", "b", "c"}}
		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetRemove(true)

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("arr")})
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		arr := m["arr"].([]any)
		x.Len(t, arr, 2)
		x.Eq(t, "a", arr[0])
		x.Eq(t, "c", arr[1])
	})
}
