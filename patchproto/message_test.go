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

func TestPatchMessage(t *testing.T) {
	a := &sample.Value{}
	a.SetS_1("foo")
	a.SetS_2("bar")

	t.Run("deleted", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109))
		d.SetDeleted(true)

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_2("bar")
		x.PbEq(t, v, b)
	})
	t.Run("assigned", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109, 309, 409))
		d.SetAssigned(dpb.String("baz"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("baz")
		v.SetS_2("bar")
		v.SetS_3("baz")
		v.SetOptS("baz")
		x.PbEq(t, v, b)
	})
	t.Run("assigned with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.SetAssigned(dpb.String("baz"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("baz")
		v.SetS_2("bar")
		v.SetS_3("baz")
		x.PbEq(t, v, b)
	})
	t.Run("assigned with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.SetAssigned(dpb.String("baz"))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("foo")
		v.SetS_2("bar")
		v.SetOptS("baz")
		x.PbEq(t, v, b)
	})
	t.Run("assigned with variant encoded number", func(t *testing.T) {
		a := &sample.Value{}
		a.SetI32_1(42)

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(105))
		d.SetAssigned(dpb.Int(36))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetI32_1(36)
		x.PbEq(t, v, b)
	})
	t.Run("assigned with zigzag encoded number", func(t *testing.T) {
		a := &sample.Value{}
		a.SetSi32_1(-42)

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(117))
		d.SetAssigned(dpb.Signed(-36))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetSi32_1(-36)
		x.PbEq(t, v, b)
	})
	t.Run("merged", func(t *testing.T) {
		b := &sample.Value{}
		b.SetS_1("foo")

		a := &sample.Value{}
		a.SetM_1(b)

		c := &sample.Value{}
		c.SetS_2("bar")

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(111))
		d.SetMerged(dpb.Message(c))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		w := &sample.Value{}
		w.SetS_1("foo")
		w.SetS_2("bar")

		v := &sample.Value{}
		v.SetM_1(w)
		x.PbEq(t, v, b)
	})
	t.Run("copied", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109, 309, 409))
		d.CopiedFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_2("bar")
		v.SetS_3("bar")
		v.SetOptS("bar")
		x.PbEq(t, v, b)
	})
	t.Run("copied with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.CopiedFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_2("bar")
		v.SetS_3("bar")
		x.PbEq(t, v, b)
	})
	t.Run("copied with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.CopiedFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("foo")
		v.SetS_2("bar")
		v.SetOptS("bar")
		x.PbEq(t, v, b)
	})
	t.Run("copied from variant encoded number to number-like", func(t *testing.T) {
		a := &sample.Value{}
		a.SetI32_1(42)

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(
			201, 202, // float
			203, 204, 205, 213, // int
			206, 207, // fixed
			215, 216, // sfixed
			217, 218, // sint
			208, // bool
			214, // enum
		))
		d.CopiedFrom(ref.Field(105))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetI32_1(42)
		v.SetF32_2(42)
		v.SetF64_2(42)
		v.SetI32_2(42)
		v.SetI64_2(42)
		v.SetU32_2(42)
		v.SetU64_2(42)
		v.SetUx32_2(42)
		v.SetUx64_2(42)
		v.SetSx32_2(42)
		v.SetSx64_2(42)
		v.SetSi32_2(42)
		v.SetSi64_2(42)
		v.SetEnum_2(42)
		v.SetB_2(true)
		x.PbEq(t, v, b)
	})
	t.Run("copied from zigzag encoded number to number-like", func(t *testing.T) {
		a := &sample.Value{}
		a.SetSi32_1(42)

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(
			201, 202, // float
			203, 204, 205, 213, // int
			206, 207, // fixed
			215, 216, // sfixed
			217, 218, // sint
			208, // bool
			214, // enum
		))
		d.CopiedFrom(ref.Field(117))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetSi32_1(42)
		v.SetF32_2(42)
		v.SetF64_2(42)
		v.SetI32_2(42)
		v.SetI64_2(42)
		v.SetU32_2(42)
		v.SetU64_2(42)
		v.SetUx32_2(42)
		v.SetUx64_2(42)
		v.SetSx32_2(42)
		v.SetSx64_2(42)
		v.SetSi32_2(42)
		v.SetSi64_2(42)
		v.SetEnum_2(42)
		v.SetB_2(true)
		x.PbEq(t, v, b)
	})
	t.Run("scattered", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109, 309, 409))
		d.ScatteredFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_3("bar")
		v.SetOptS("bar")
		x.PbEq(t, v, b)
	})
	t.Run("scattered with no insert", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoInsert(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.ScatteredFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_3("bar")
		x.PbEq(t, v, b)
	})
	t.Run("scattered with no update", func(t *testing.T) {
		d := &dpb.Entry{}
		d.SetNoUpdate(true)
		d.AppendTargets(target.Fields(109, 309, 409))
		d.ScatteredFrom(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("foo")
		v.SetOptS("bar")
		x.PbEq(t, v, b)
	})
	t.Run("swapped", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109))
		d.SwappedWith(ref.Field(209))

		b, err := patchproto.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_1("bar")
		v.SetS_2("foo")
		x.PbEq(t, v, b)
	})
	// // t.Run("edited", func(t *testing.T) {})
	t.Run("nested", func(t *testing.T) {
		b := &sample.Value{}
		b.SetS_1("foo")

		a := &sample.Value{}
		a.SetM_1(b)

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(209))
		d.SetAssigned(dpb.String("bar"))

		d_ := &dpb.Entry{}
		d_.AppendTargets(target.Fields(111))
		d_.SetNested(dpb.NewDelta(d))

		b, err := patchproto.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		w := &sample.Value{}
		w.SetS_1("foo")
		w.SetS_2("bar")

		v := &sample.Value{}
		v.SetM_1(w)
		x.PbEq(t, v, b)
	})
}
