package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

func mapEntry(inner *dpb.Entry) *dpb.Entry {
	outer := &dpb.Entry{}
	outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10909))}) // m_s_s
	outer.SetNest(dpb.NewDelta(inner))
	return outer
}

func TestPatchMap(t *testing.T) {
	a := &sample.Value{}
	a.SetMSS(map[string]string{
		"A": "a",
		"B": "b",
		"C": "c",
	})
	t.Run("remove existing keys", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("B"), dpb.SegName("D")}) // D does not exist
		d.SetRemove(true)

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"C": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("test pass", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("A")})
		d.SetTest(dpb.ValS("a"))

		_, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)
	})
	t.Run("test fail", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("A")})
		d.SetTest(dpb.ValS("wrong"))

		_, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.Error(t, err)
	})
	t.Run("insert absent key", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("D")}) // D does not exist
		d.SetInsert(dpb.ValS("d"))

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{"A": "a", "B": "b", "C": "c", "D": "d"})
		x.PbEq(t, v, b)
	})
	t.Run("insert present key is no-op", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("A")}) // A already exists
		d.SetInsert(dpb.ValS("new"))

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		x.PbEq(t, a, b)
	})
	t.Run("assign creates and updates", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("B"), dpb.SegName("D")})
		d.SetAssign(dpb.ValS("z"))

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "z",
			"C": "c",
			"D": "z", // D is created
		})
		x.PbEq(t, v, b)
	})
	t.Run("move renames key", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("Z")}) // destination
		d.SetMove(dpb.Field("A"))                      // source

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"B": "b",
			"C": "c",
			"Z": "a",
		})
		x.PbEq(t, v, b)
	})
	t.Run("copy duplicates value", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegName("Z")}) // destination
		d.SetCopy(dpb.Field("A"))                      // source

		b, err := patchproto.Patched(a, dpb.NewDelta(mapEntry(d)))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a", // source kept
			"B": "b",
			"C": "c",
			"Z": "a",
		})
		x.PbEq(t, v, b)
	})
	t.Run("nest into message value", func(t *testing.T) {
		inner := &sample.Value{}
		inner.SetS_1("hello")

		aa := &sample.Value{}
		aa.SetMSM(map[string]*sample.Value{"key": inner})

		field_d := &dpb.Entry{}
		field_d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(209))}) // s_2
		field_d.SetAssign(dpb.ValS("world"))

		map_d := &dpb.Entry{}
		map_d.SetTargets([]*dpb.Segment{dpb.SegName("key")})
		map_d.SetNest(dpb.NewDelta(field_d))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10911))}) // m_s_m
		outer.SetNest(dpb.NewDelta(map_d))

		b, err := patchproto.Patched(aa, dpb.NewDelta(outer))
		x.NoError(t, err)

		want_inner := &sample.Value{}
		want_inner.SetS_1("hello")
		want_inner.SetS_2("world")
		want := &sample.Value{}
		want.SetMSM(map[string]*sample.Value{"key": want_inner})
		x.PbEq(t, want, b)
	})
}
