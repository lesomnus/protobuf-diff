package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

func listEntry(inner *dpb.Entry) *dpb.Entry {
	outer := &dpb.Entry{}
	outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1009))}) // r_s_1
	outer.SetNest(dpb.NewDelta(inner))
	return outer
}

func TestPatchList(t *testing.T) {
	a := &sample.Value{}
	a.SetRS_1([]string{"foo", "bar", "baz"})
	t.Run("remove", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		d.SetRemove(true)

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar"})
		x.PbEq(t, v, b)
	})
	t.Run("remove with negative indices", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(-3), dpb.SegIndex(-1)})
		d.SetRemove(true)

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar"})
		x.PbEq(t, v, b)
	})
	t.Run("test pass", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0)})
		d.SetTest(dpb.ValS("foo"))

		_, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)
	})
	t.Run("test fail", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0)})
		d.SetTest(dpb.ValS("wrong"))

		_, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.Error(t, err)
	})
	t.Run("assign", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		d.SetAssign(dpb.ValS("z"))

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "bar", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("assign with negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(-3), dpb.SegIndex(-1)})
		d.SetAssign(dpb.ValS("z"))

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "bar", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("insert before index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		d.SetInsert(dpb.ValS("z"))

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "foo", "bar", "z", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("insert at -1 appends", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(-1)})
		d.SetInsert(dpb.ValS("qux"))

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "bar", "baz", "qux"})
		x.PbEq(t, v, b)
	})
	t.Run("insert with negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(-3), dpb.SegIndex(-1)})
		d.SetInsert(dpb.ValS("z"))

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "z", "bar", "baz", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("copy replaces at target indices", func(t *testing.T) {
		// copy element at index 1 ("bar") to positions 0 and 2
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
		d.SetCopy(dpb.FieldNum(1)) // source index 1

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		// splice inserts "bar" before index 0 and 2: [bar, foo, bar, bar, baz]
		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "foo", "bar", "bar", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("move forward", func(t *testing.T) {
		// move element at index 0 ("foo") to insert before index 2
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(2)})
		d.SetMove(dpb.FieldNum(0)) // source index 0

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		// insert "foo" before index 2: [foo, bar, foo, baz], remove original at 0: [bar, foo, baz]
		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "foo", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("move backward", func(t *testing.T) {
		// move element at index 2 ("baz") to insert before index 0
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegIndex(0)})
		d.SetMove(dpb.FieldNum(2)) // source index 2

		b, err := patchproto.Patched(a, dpb.NewDelta(listEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"baz", "foo", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("nest into message element", func(t *testing.T) {
		m0 := &sample.Value{}
		m0.SetS_1("foo")
		m1 := &sample.Value{}
		m1.SetS_1("bar")

		aa := &sample.Value{}
		aa.SetRM_1([]*sample.Value{m0, m1})

		// Patch r_m_1[1].s_2 = "baz"
		field_d := &dpb.Entry{}
		field_d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(209))}) // s_2
		field_d.SetAssign(dpb.ValS("baz"))

		list_d := &dpb.Entry{}
		list_d.SetTargets([]*dpb.Segment{dpb.SegIndex(1)})
		list_d.SetNest(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1011))}) // r_m_1
		outer.SetNest(dpb.NewDelta(list_d))

		b, err := patchproto.Patched(aa, dpb.NewDelta(outer))
		x.NoError(t, err)

		w0 := &sample.Value{}
		w0.SetS_1("foo")
		w1 := &sample.Value{}
		w1.SetS_1("bar")
		w1.SetS_2("baz")

		want := &sample.Value{}
		want.SetRM_1([]*sample.Value{w0, w1})
		x.PbEq(t, want, b)
	})
}
