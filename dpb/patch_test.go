package dpb_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
		x.NoError(t, err)

		v := &sample.Value{}
		v.SetS_2("bar")
		x.PbEq(t, v, b)
	})
	t.Run("assigned", func(t *testing.T) {
		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(109, 309, 409))
		d.SetAssigned(dpb.String("baz"))

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
		x.NoError(t, err)

		w := &sample.Value{}
		w.SetS_1("foo")
		w.SetS_2("bar")

		v := &sample.Value{}
		v.SetM_1(w)
		x.PbEq(t, v, b)
	})
}

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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_outer))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_outer))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_))
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

		b, err := dpb.Patched(a, dpb.NewDelta(d_outer))
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

func TeatPatchPath(t *testing.T) {
}
