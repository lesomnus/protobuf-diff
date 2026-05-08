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

func TestPatchMap(t *testing.T) {
	a := &sample.Value{}
	a.SetMSS(map[string]string{
		"A": "a",
		"B": "b",
		"C": "c",
	})

	t.Run("deleted", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B", "D"))
		d.SetDeleted(true)

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"C": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("assigned", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B", "D"))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "z",
			"C": "c",
			"D": "z",
		})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "z",
			"C": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("assigned with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.SetAssigned(dpb.String("z"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "b",
			"C": "c",
			"D": "z",
		})
		x.PbEq(t, v, b)
	})
	t.Run("copied", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B", "D"))
		d.CopiedFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "c",
			"C": "c",
			"D": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("copied with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.CopiedFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "c",
			"C": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("copied with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.CopiedFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "b",
			"C": "c",
			"D": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("scattered", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B", "D"))
		d.ScatteredFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "c",
			"D": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.ScatteredFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("scattered with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.StringKeys("B", "D"))
		d.ScatteredFrom(ref.StringKey("C"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "a",
			"B": "b",
			"D": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("swapped", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.StringKeys("B"))
		d.SwappedWith(ref.StringKey("A"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(10909))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetMSS(map[string]string{
			"A": "b",
			"B": "a",
			"C": "c",
		})
		x.PbEq(t, v, b)
	})
	t.Run("nested", func(t *testing.T) {
		ma := &sample.Value{}
		ma.SetS_1("foo")
		mb := &sample.Value{}
		mb.SetS_1("bar")

		a := &sample.Value{}
		a.SetMSM(map[string]*sample.Value{
			"A": ma,
			"B": mb,
		})

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(209))
		d.SetAssigned(dpb.String("baz"))

		d_inner := &dpb.Entry{}
		d_inner.AppendTargets(target.StringKeys("B", "D"))
		d_inner.SetNested(dpb.NewDelta(d))

		d_outer := &dpb.Entry{}
		d_outer.AppendTargets(target.Fields(10911))
		d_outer.SetNested(dpb.NewDelta(d_inner))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_outer))
		x.NoError(t, err)

		wa := &sample.Value{}
		wa.SetS_1("foo")
		wb := &sample.Value{}
		wb.SetS_1("bar")
		wb.SetS_2("baz")
		wd := &sample.Value{}
		wd.SetS_2("baz")

		v := &sample.Value{}
		v.SetMSM(map[string]*sample.Value{
			"A": wa,
			"B": wb,
		})
		x.PbEq(t, v, b)
	})
}
