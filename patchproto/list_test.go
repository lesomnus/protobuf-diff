package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func TestPatchList(t *testing.T) {
	a := &sample.Value{}
	a.SetRS_1([]string{"foo", "bar", "baz"})

	t.Run("deleted", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(0, 2))
		d.SetDeleted(true)

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar"})
		x.PbEq(t, v, b)
	})
	t.Run("deleted with negative indices", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(-3, -1))
		d.SetDeleted(true)

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar"})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with update mode", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(0, 2))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "bar", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with update mode and negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(-3, -1))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "bar", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with insert mode", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(0, 2))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"z", "foo", "bar", "z", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with insert mode and negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(-3, -1))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "z", "bar", "baz", "z"})
		x.PbEq(t, v, b)
	})
	t.Run("copied with update mode", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(0, 2))
		d.CopiedFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "bar", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("copied with update mode and negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(-3, -1))
		d.CopiedFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "bar", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("copied with insert mode", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(0, 2))
		d.CopiedFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "foo", "bar", "bar", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("copied with insert mode and negative index", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(-3, -1))
		d.CopiedFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "bar", "bar", "baz", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with update mode", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz", "qux"})

		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(0, 3))
		d.ScatteredFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "baz", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with update mode and negative index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz", "qux"})

		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(-3, -1))
		d.ScatteredFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "baz", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with insert mode", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz", "qux"})

		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(0, 3))
		d.ScatteredFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "foo", "baz", "bar", "qux"})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with insert mode and negative index", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar", "baz", "qux"})

		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Indices(-3, -1))
		d.ScatteredFrom(ref.Index(1))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"foo", "bar", "baz", "qux", "bar"})
		x.PbEq(t, v, b)
	})
	t.Run("swapped", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(2))
		d.SwappedWith(ref.Index(0))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"baz", "bar", "foo"})
		x.PbEq(t, v, b)
	})
	t.Run("swapped with negative indices", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Indices(-2))
		d.SwappedWith(ref.Index(-3))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(1009))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetRS_1([]string{"bar", "foo", "baz"})
		x.PbEq(t, v, b)
	})
	t.Run("nested with update mode", func(t *testing.T) {
		m0 := &sample.Value{}
		m0.SetS_1("foo")
		m1 := &sample.Value{}
		m1.SetS_1("bar")

		a := &sample.Value{}
		a.SetRM_1([]*sample.Value{m0, m1})

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(209))
		d.CopiedFrom(ref.Field(109))

		d_inner := &dpb.Entry{}
		d_inner.AppendTargets(target.Indices(1))
		d_inner.SetNested(dpb.NewDelta(d))

		d_outer := &dpb.Entry{}
		d_outer.AppendTargets(target.Fields(1011))
		d_outer.SetNested(dpb.NewDelta(d_inner))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_outer))
		x.NoError(t, err)

		w0 := &sample.Value{}
		w0.SetS_1("foo")
		w1 := &sample.Value{}
		w1.SetS_1("bar")
		w1.SetS_2("bar")

		v := &sample.Value{}
		v.SetRM_1([]*sample.Value{w0, w1})
		x.PbEq(t, v, b)
	})
	t.Run("nested with insert mode", func(t *testing.T) {
		m0 := &sample.Value{}
		m0.SetS_1("foo")
		m1 := &sample.Value{}
		m1.SetS_1("bar")

		a := &sample.Value{}
		a.SetRM_1([]*sample.Value{m0, m1})

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109))
		d.SetAssigned(dpb.String("baz"))

		d_inner := &dpb.Entry{}
		d_inner.SetNoUpdate(true)
		d_inner.AppendTargets(target.Indices(1))
		d_inner.SetNested(dpb.NewDelta(d))

		d_outer := &dpb.Entry{}
		d_outer.AppendTargets(target.Fields(1011))
		d_outer.SetNested(dpb.NewDelta(d_inner))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_outer))
		x.NoError(t, err)

		w0 := &sample.Value{}
		w0.SetS_1("foo")
		w1 := &sample.Value{}
		w1.SetS_1("baz")
		w2 := &sample.Value{}
		w2.SetS_1("bar")

		v := &sample.Value{}
		v.SetRM_1([]*sample.Value{w0, w1, w2})
		x.PbEq(t, v, b)
	})
}
