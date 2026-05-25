package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

func TestPatchMessage(t *testing.T) {
	a := &sample.Value{}
	a.SetS_1("foo")
	a.SetS_2("bar")

	t.Run("remove", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetRemove(true)

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_2("bar")
		x.PbEq(t, v, b)
	})
	t.Run("test pass", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetTest(dpb.ValS("foo"))

		_, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)
	})
	t.Run("test fail", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetTest(dpb.ValS("wrong"))

		_, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.Error(t, err)
	})
	t.Run("insert absent field", func(t *testing.T) {
		// opt_s is absent; insert should set it
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(409))}) // opt_s
		d.SetInsert(dpb.ValS("baz"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("foo")
		v.SetS_2("bar")
		v.SetOptS("baz")
		x.PbEq(t, v, b)
	})
	t.Run("insert present optional field is no-op", func(t *testing.T) {
		// opt_s has presence tracking; insert on an already-set field should be a no-op
		aa := &sample.Value{}
		aa.SetOptS("existing")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(409))}) // opt_s
		d.SetInsert(dpb.ValS("new"))

		b, err := patchproto.Patched(aa, dpb.NewDelta(d))
		x.NoError(t, err)

		x.PbEq(t, aa, b)
	})
	t.Run("assign", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{
			dpb.SegField(dpb.FieldNum(109)), // s_1
			dpb.SegField(dpb.FieldNum(309)), // s_3
			dpb.SegField(dpb.FieldNum(409)), // opt_s
		})
		d.SetAssign(dpb.ValS("baz"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("baz")
		v.SetS_2("bar")
		v.SetS_3("baz")
		v.SetOptS("baz")
		x.PbEq(t, v, b)
	})
	t.Run("assign int field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetI32_1(42)

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(105))}) // i32_1
		d.SetAssign(dpb.ValI(36))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetI32_1(36)
		x.PbEq(t, v, b)
	})
	t.Run("move", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetMove(dpb.FieldNum(209))                                  // from s_2

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		// s_2 is cleared by move
		x.PbEq(t, v, b)
	})
	t.Run("copy", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{
			dpb.SegField(dpb.FieldNum(109)), // s_1
			dpb.SegField(dpb.FieldNum(309)), // s_3
		})
		d.SetCopy(dpb.FieldNum(209)) // from s_2

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_2("bar") // s_2 stays (copy keeps source)
		v.SetS_3("bar")
		x.PbEq(t, v, b)
	})
	t.Run("nest into message field", func(t *testing.T) {
		inner := &sample.Value{}
		inner.SetS_1("foo")

		aa := &sample.Value{}
		aa.SetM_1(inner)

		// Patch m_1.s_2 = "bar" using nest
		sub := &dpb.Entry{}
		sub.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(209))}) // s_2
		sub.SetAssign(dpb.ValS("bar"))

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(111))}) // m_1
		d.SetNest(dpb.NewDelta(sub))

		b, err := patchproto.Patched(aa, dpb.NewDelta(d))
		x.NoError(t, err)

		want_inner := &sample.Value{}
		want_inner.SetS_1("foo")
		want_inner.SetS_2("bar")
		want := &sample.Value{}
		want.SetM_1(want_inner)
		x.PbEq(t, want, b)
	})
	t.Run("unknown field number is silently skipped", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(9999))})
		d.SetAssign(dpb.ValS("bar"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)
		x.PbEq(t, a, b)
	})
}
