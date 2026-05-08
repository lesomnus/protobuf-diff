package patchstruct_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchstruct"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

type Inner struct {
	X string
	Y int32
}

type Sample struct {
	S1   string
	S2   string
	I1   int32
	I2   int32
	F1   float32
	F2   float64
	B1   bool
	Opt  *string
	Opt2 *string
	M1   Inner
	Sl   []string
	Mp   map[string]string
}

func mustPatch(t *testing.T, v any, delta *dpb.Delta) {
	t.Helper()
	if err := patchstruct.Patch(v, delta); err != nil {
		t.Fatalf("Patch: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestPatchStruct(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("S1"))
		d.SetDeleted(true)

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "", v.S1)
		x.Eq(t, "bar", v.S2)
	})
	t.Run("assigned string", func(t *testing.T) {
		v := &Sample{S1: "foo"}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("S1", "S2"))
		d.SetAssigned(dpb.String("baz"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "baz", v.S1)
		x.Eq(t, "baz", v.S2)
	})
	t.Run("assigned int", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("I1"))
		d.SetAssigned(dpb.Int(42))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, int32(42), v.I1)
	})
	t.Run("assigned float32", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("F1"))
		d.SetAssigned(dpb.Float(3.14))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, float32(3.14), v.F1)
	})
	t.Run("assigned float64", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("F2"))
		d.SetAssigned(dpb.Double(2.718))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, 2.718, v.F2)
	})
	t.Run("assigned bool", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B1"))
		d.SetAssigned(dpb.Bool(true))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, true, v.B1)
	})
	t.Run("assigned with no_insert skips nil pointer", func(t *testing.T) {
		v := &Sample{S1: "foo"}
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.StringKeys("S1", "Opt"))
		d.SetAssigned(dpb.String("baz"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "baz", v.S1)
		x.Eq(t, nil, v.Opt)
	})
	t.Run("assigned with no_update skips existing pointer", func(t *testing.T) {
		v := &Sample{S1: "foo", Opt: strPtr("existing")}
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.StringKeys("S1", "Opt"))
		d.SetAssigned(dpb.String("baz"))

		mustPatch(t, v, dpb.NewDelta(d))
		// S1 is non-pointer, always present → skipped by no_update
		x.Eq(t, "foo", v.S1)
		// Opt is non-nil → skipped by no_update
		x.NotNil(t, v.Opt)
		x.Eq(t, "existing", *v.Opt)
	})
	t.Run("assigned pointer field", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Opt"))
		d.SetAssigned(dpb.String("hello"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.NotNil(t, v.Opt)
		x.Eq(t, "hello", *v.Opt)
	})
	t.Run("copied", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("S1"))
		d.CopiedFrom(ref.StringKey("S2"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
		x.Eq(t, "bar", v.S2)
	})
	t.Run("scattered", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("S1"))
		d.ScatteredFrom(ref.StringKey("S2"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
		x.Eq(t, "", v.S2)
	})
	t.Run("swapped", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("S1"))
		d.SwappedWith(ref.StringKey("S2"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
		x.Eq(t, "foo", v.S2)
	})
	t.Run("nested struct", func(t *testing.T) {
		v := &Sample{M1: Inner{X: "a"}}
		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("Y"))
		inner.SetAssigned(dpb.Int(99))

		outer := &dpb.Entry{}
		outer.AppendTargets(target.StringKeys("M1"))
		outer.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "a", v.M1.X)
		x.Eq(t, 99, v.M1.Y)
	})
	t.Run("path-based target", func(t *testing.T) {
		v := &Sample{S1: "foo"}
		d := &dpb.Entry{}
		d.SetPath(dpb.P.S("S1").Value())
		d.SetAssigned(dpb.String("bar"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
	})
}

func TestPatchSlice(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Sl"))

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(1))
		inner.SetDeleted(true)
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Len(t, v.Sl, 2)
		x.Eq(t, "a", v.Sl[0])
		x.Eq(t, "c", v.Sl[1])
	})
	t.Run("assigned", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Sl"))
		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(1))
		inner.SetAssigned(dpb.String("z"))
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "z", v.Sl[1])
	})
	t.Run("inserted with no_update", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Sl"))

		inner := &dpb.Entry{}
		inner.SetNoUpdate(true)
		inner.AppendTargets(target.Indices(-1))
		inner.SetAssigned(dpb.String("c"))
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Len(t, v.Sl, 3)
		x.Eq(t, "c", v.Sl[2])
	})
	t.Run("swapped", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Sl"))

		inner := &dpb.Entry{}
		inner.AppendTargets(target.Indices(0))
		inner.SwappedWith(ref.Index(2))
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "c", v.Sl[0])
		x.Eq(t, "b", v.Sl[1])
		x.Eq(t, "a", v.Sl[2])
	})
}

func TestPatchMap(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1", "b": "2"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Mp"))

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("a"))
		inner.SetDeleted(true)
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		if _, ok := v.Mp["a"]; ok {
			t.Fatal("expected key 'a' deleted")
		}
		x.Eq(t, "2", v.Mp["b"])
	})
	t.Run("assigned", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Mp"))

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("a", "b"))
		inner.SetAssigned(dpb.String("z"))
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "z", v.Mp["a"])
		x.Eq(t, "z", v.Mp["b"])
	})
	t.Run("swapped", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1", "b": "2"}}
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("Mp"))
		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("a"))
		inner.SwappedWith(ref.StringKey("b"))
		d.SetNested(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "2", v.Mp["a"])
		x.Eq(t, "1", v.Mp["b"])
	})
}
