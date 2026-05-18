package patchjson_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchjson"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func mustPatch(t *testing.T, v any, delta *dpb.Delta) {
	t.Helper()
	if err := patchjson.Patch(v, delta); err != nil {
		t.Fatalf("Patch: %v", err)
	}
}

func TestPatchMap(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SetDeleted(true)
		mustPatch(t, m, dpb.NewDelta(e))
		if _, ok := m["a"]; ok {
			t.Fatal("expected key 'a' deleted")
		}
		x.Eq(t, "2", m["b"])
	})
	t.Run("assigned", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a", "b"))
		e.SetAssigned(patchjson.Value("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "new", m["a"])
		x.Eq(t, "new", m["b"])
	})
	t.Run("assigned null", func(t *testing.T) {
		m := map[string]any{"a": "v"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SetAssigned(patchjson.Value(nil))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, nil, m["a"])
	})
	t.Run("assigned number", func(t *testing.T) {
		m := map[string]any{}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("n"))
		e.SetAssigned(patchjson.Value(float64(3.14)))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, 3.14, m["n"])
	})
	t.Run("assigned bool", func(t *testing.T) {
		m := map[string]any{}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("ok"))
		e.SetAssigned(patchjson.Value(true))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, true, m["ok"])
	})
	t.Run("no_insert skips missing key", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.SetNoInsert(true)
		e.AppendTargets(target.StringKeys("a", "b"))
		e.SetAssigned(patchjson.Value("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "new", m["a"])
		if _, ok := m["b"]; ok {
			t.Fatal("expected key 'b' not inserted")
		}
	})
	t.Run("no_update skips existing key", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.SetNoUpdate(true)
		e.AppendTargets(target.StringKeys("a", "b"))
		e.SetAssigned(patchjson.Value("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "old", m["a"])
		x.Eq(t, "new", m["b"])
	})
	t.Run("copied", func(t *testing.T) {
		m := map[string]any{"src": "hello", "dst": "old"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("dst"))
		e.CopiedFrom(ref.StringKey("src"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "hello", m["src"])
		x.Eq(t, "hello", m["dst"])
	})
	t.Run("scattered", func(t *testing.T) {
		m := map[string]any{"src": "hello", "dst": "old"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("dst"))
		e.ScatteredFrom(ref.StringKey("src"))
		mustPatch(t, m, dpb.NewDelta(e))
		if _, ok := m["src"]; ok {
			t.Fatal("expected src deleted after scatter")
		}
		x.Eq(t, "hello", m["dst"])
	})
	t.Run("swapped", func(t *testing.T) {
		m := map[string]any{"a": "1", "b": "2"}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("a"))
		e.SwappedWith(ref.StringKey("b"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "2", m["a"])
		x.Eq(t, "1", m["b"])
	})
	t.Run("nested", func(t *testing.T) {
		m := map[string]any{"obj": map[string]any{"x": "old"}}
		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("x"))
		inner.SetAssigned(patchjson.Value("new"))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("obj"))
		outer.SetNested(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		x.Eq(t, "new", m["obj"].(map[string]any)["x"])
	})
	t.Run("path-based target", func(t *testing.T) {
		m := map[string]any{"a": "old"}
		e := &dpb.Entry{}
		e.SetPath(dpb.P.S("a").Value())
		e.SetAssigned(patchjson.Value("new"))
		mustPatch(t, m, dpb.NewDelta(e))
		x.Eq(t, "new", m["a"])
	})
	t.Run("merged", func(t *testing.T) {
		m := map[string]any{"obj": map[string]any{"x": "1", "y": "2"}}
		e := &dpb.Entry{}
		e.AppendTargets(target.StringKeys("obj"))
		e.SetMerged(patchjson.Value(map[string]any{"y": "99", "z": "3"}))
		mustPatch(t, m, dpb.NewDelta(e))
		obj := m["obj"].(map[string]any)
		x.Eq(t, "1", obj["x"])
		x.Eq(t, "99", obj["y"])
		x.Eq(t, "3", obj["z"])
	})
}

func TestPatchList(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(1))
		e.SetDeleted(true)
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 2)
		x.Eq(t, "a", (*sl)[0])
		x.Eq(t, "c", (*sl)[1])
	})
	t.Run("assigned update", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(1))
		e.SetAssigned(patchjson.Value("z"))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Eq(t, "z", (*sl)[1])
	})
	t.Run("assigned insert with no_update", func(t *testing.T) {
		sl := &[]any{"a", "b"}
		e := &dpb.Entry{}
		e.SetNoUpdate(true)
		e.AppendTargets(target.Indices(-1))
		e.SetAssigned(patchjson.Value("c"))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 3)
		x.Eq(t, "c", (*sl)[2])
	})
	t.Run("copied", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(0))
		e.CopiedFrom(ref.Index(2))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Eq(t, "c", (*sl)[0])
		x.Eq(t, "c", (*sl)[2])
	})
	t.Run("scattered", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(0))
		e.ScatteredFrom(ref.Index(2))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Len(t, *sl, 2)
		x.Eq(t, "c", (*sl)[0])
		x.Eq(t, "b", (*sl)[1])
	})
	t.Run("swapped", func(t *testing.T) {
		sl := &[]any{"a", "b", "c"}
		e := &dpb.Entry{}
		e.AppendTargets(target.Indices(0))
		e.SwappedWith(ref.Index(2))
		mustPatch(t, sl, dpb.NewDelta(e))
		x.Eq(t, "c", (*sl)[0])
		x.Eq(t, "b", (*sl)[1])
		x.Eq(t, "a", (*sl)[2])
	})
	t.Run("nested list-in-map", func(t *testing.T) {
		m := map[string]any{"arr": []any{"a", "b", "c"}}
		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(1))
		inner.SetAssigned(patchjson.Value("z"))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("arr"))
		outer.SetNested(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		x.Eq(t, "z", m["arr"].([]any)[1])
	})
	t.Run("nested delete in list-in-map", func(t *testing.T) {
		m := map[string]any{"arr": []any{"a", "b", "c"}}
		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(1))
		inner.SetDeleted(true)

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("arr"))
		outer.SetNested(dpb.NewDelta(inner))

		mustPatch(t, m, dpb.NewDelta(outer))
		arr := m["arr"].([]any)
		x.Len(t, arr, 2)
		x.Eq(t, "a", arr[0])
		x.Eq(t, "c", arr[1])
	})
}
