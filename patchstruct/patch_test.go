package patchstruct_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchstruct"
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
	t.Run("remove", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("S1")})
		d.SetRemove(true)

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "", v.S1)
		x.Eq(t, "bar", v.S2)
	})
	t.Run("assign string", func(t *testing.T) {
		v := &Sample{S1: "foo"}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("S1"), dpb.SegName("S2")})
		d.SetAssign(dpb.ValS("baz"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "baz", v.S1)
		x.Eq(t, "baz", v.S2)
	})
	t.Run("assign int", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("I1")})
		d.SetAssign(dpb.ValI(42))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, int32(42), v.I1)
	})
	t.Run("assign float32", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("F1")})
		d.SetAssign(dpb.ValF(3.14))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, float32(3.14), v.F1)
	})
	t.Run("assign float64", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("F2")})
		d.SetAssign(dpb.ValF(2.718))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, 2.718, v.F2)
	})
	t.Run("assign bool", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("B1")})
		d.SetAssign(dpb.ValB(true))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, true, v.B1)
	})
	t.Run("assign pointer field", func(t *testing.T) {
		v := &Sample{}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("Opt")})
		d.SetAssign(dpb.ValS("hello"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.NotNil(t, v.Opt)
		x.Eq(t, "hello", *v.Opt)
	})
	t.Run("insert absent pointer field", func(t *testing.T) {
		v := &Sample{S1: "foo"}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("Opt")})
		d.SetInsert(dpb.ValS("baz"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.NotNil(t, v.Opt)
		x.Eq(t, "baz", *v.Opt)
	})
	t.Run("insert present pointer field is no-op", func(t *testing.T) {
		v := &Sample{Opt: strPtr("existing")}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("Opt")})
		d.SetInsert(dpb.ValS("new"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.NotNil(t, v.Opt)
		x.Eq(t, "existing", *v.Opt)
	})
	t.Run("move renames field", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("S1")})
		d.SetMove(dpb.Field("S2"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
		x.Eq(t, "", v.S2)
	})
	t.Run("copy duplicates field", func(t *testing.T) {
		v := &Sample{S1: "foo", S2: "bar"}
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("S1")})
		d.SetCopy(dpb.Field("S2"))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, "bar", v.S1)
		x.Eq(t, "bar", v.S2)
	})
	t.Run("nest into struct field", func(t *testing.T) {
		v := &Sample{M1: Inner{X: "a"}}
		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("Y")})
		inner.SetAssign(dpb.ValI(99))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("M1")})
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "a", v.M1.X)
		x.Eq(t, int32(99), v.M1.Y)
	})
	t.Run("path navigation", func(t *testing.T) {
		v := &Sample{M1: Inner{X: "a"}}
		d := &dpb.Entry{}
		d.SetPath(dpb.PathOf(dpb.Field("M1")))
		d.SetTargets([]*dpb.Segment{dpb.SegName("Y")})
		d.SetAssign(dpb.ValI(42))

		mustPatch(t, v, dpb.NewDelta(d))
		x.Eq(t, int32(42), v.M1.Y)
	})
}

func TestPatchSlice(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Sl")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetRemove(true)
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Len(t, v.Sl, 2)
		x.Eq(t, "a", v.Sl[0])
		x.Eq(t, "c", v.Sl[1])
	})
	t.Run("assign", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Sl")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetAssign(dpb.ValS("z"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "z", v.Sl[1])
	})
	t.Run("insert appends at -1", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Sl")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(-1)})
		inner.SetInsert(dpb.ValS("c"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Len(t, v.Sl, 3)
		x.Eq(t, "c", v.Sl[2])
	})
	t.Run("copy splices element", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Sl")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		inner.SetCopy(dpb.FieldNum(0))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Len(t, v.Sl, 4)
		x.Eq(t, "a", v.Sl[0])
		x.Eq(t, "a", v.Sl[1])
		x.Eq(t, "b", v.Sl[2])
		x.Eq(t, "c", v.Sl[3])
	})
	t.Run("move to end", func(t *testing.T) {
		v := &Sample{Sl: []string{"a", "b", "c"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Sl")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(-1)}) // append
		inner.SetMove(dpb.FieldNum(0))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Len(t, v.Sl, 3)
		x.Eq(t, "b", v.Sl[0])
		x.Eq(t, "c", v.Sl[1])
		x.Eq(t, "a", v.Sl[2])
	})
}

func TestPatchMap(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1", "b": "2"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Mp")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("a")})
		inner.SetRemove(true)
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		if _, ok := v.Mp["a"]; ok {
			t.Fatal("expected key 'a' deleted")
		}
		x.Eq(t, "2", v.Mp["b"])
	})
	t.Run("assign", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Mp")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("a"), dpb.SegName("b")})
		inner.SetAssign(dpb.ValS("z"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "z", v.Mp["a"])
		x.Eq(t, "z", v.Mp["b"])
	})
	t.Run("insert absent key", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Mp")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("a"), dpb.SegName("b")})
		inner.SetInsert(dpb.ValS("new"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "1", v.Mp["a"])   // unchanged
		x.Eq(t, "new", v.Mp["b"]) // inserted
	})
	t.Run("move renames key", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1", "b": "2"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Mp")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("c")})
		inner.SetMove(dpb.Field("a"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		if _, ok := v.Mp["a"]; ok {
			t.Fatal("expected key 'a' removed")
		}
		x.Eq(t, "1", v.Mp["c"])
		x.Eq(t, "2", v.Mp["b"])
	})
	t.Run("copy duplicates key", func(t *testing.T) {
		v := &Sample{Mp: map[string]string{"a": "1", "b": "2"}}
		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegName("Mp")})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("c")})
		inner.SetCopy(dpb.Field("a"))
		outer.SetNest(dpb.NewDelta(inner))

		mustPatch(t, v, dpb.NewDelta(outer))
		x.Eq(t, "1", v.Mp["a"])
		x.Eq(t, "1", v.Mp["c"])
	})
}
