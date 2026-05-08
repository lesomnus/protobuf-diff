package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

// diffRoundtrip checks that Patch(from, Diff(from, to)) == to.
func diffRoundtrip(t *testing.T, from, to *sample.Value) {
	t.Helper()

	delta, err := patchproto.Diff(from, to)
	x.NoError(t, err)

	got, err := patchproto.Patched(from, delta)
	x.NoError(t, err)
	x.PbEq(t, to, got)
}

func TestDiffMessage(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		diffRoundtrip(t, a, a)
	})
	t.Run("scalar assigned", func(t *testing.T) {
		from := &sample.Value{}
		from.SetS_1("foo")

		to := &sample.Value{}
		to.SetS_1("bar")

		diffRoundtrip(t, from, to)
	})
	t.Run("scalar deleted", func(t *testing.T) {
		from := &sample.Value{}
		from.SetS_1("foo")

		diffRoundtrip(t, from, &sample.Value{})
	})
	t.Run("scalar inserted", func(t *testing.T) {
		to := &sample.Value{}
		to.SetS_1("foo")

		diffRoundtrip(t, &sample.Value{}, to)
	})
	t.Run("multiple scalars", func(t *testing.T) {
		from := &sample.Value{}
		from.SetS_1("foo")
		from.SetS_2("bar")
		from.SetI32_1(10)

		to := &sample.Value{}
		to.SetS_1("baz")
		to.SetI32_1(10)
		to.SetI32_2(99)

		diffRoundtrip(t, from, to)
	})
	t.Run("all numeric kinds", func(t *testing.T) {
		from := &sample.Value{}
		from.SetF64_1(1.0)
		from.SetF32_1(2.0)
		from.SetI64_1(3)
		from.SetU64_1(4)
		from.SetI32_1(5)
		from.SetUx64_1(6)
		from.SetUx32_1(7)
		from.SetB_1(true)
		from.SetSi32_1(-8)
		from.SetSi64_1(-9)
		from.SetSx32_1(-10)
		from.SetSx64_1(-11)

		to := &sample.Value{}
		to.SetF64_1(10.0)
		to.SetF32_1(20.0)
		to.SetI64_1(30)
		to.SetU64_1(40)
		to.SetI32_1(50)
		to.SetUx64_1(60)
		to.SetUx32_1(70)
		to.SetB_1(false)
		to.SetSi32_1(-80)
		to.SetSi64_1(-90)
		to.SetSx32_1(-100)
		to.SetSx64_1(-110)

		diffRoundtrip(t, from, to)
	})
	t.Run("optional field inserted", func(t *testing.T) {
		to := &sample.Value{}
		to.SetOptS("hello")

		diffRoundtrip(t, &sample.Value{}, to)
	})
	t.Run("optional field deleted", func(t *testing.T) {
		from := &sample.Value{}
		from.SetOptS("hello")

		diffRoundtrip(t, from, &sample.Value{})
	})
	t.Run("nested message assigned", func(t *testing.T) {
		sub := &sample.Value{}
		sub.SetS_1("inner")

		to := &sample.Value{}
		to.SetM_1(sub)

		diffRoundtrip(t, &sample.Value{}, to)
	})
	t.Run("nested message diff", func(t *testing.T) {
		sub1 := &sample.Value{}
		sub1.SetS_1("old")

		from := &sample.Value{}
		from.SetM_1(sub1)

		sub2 := &sample.Value{}
		sub2.SetS_1("new")
		sub2.SetS_2("extra")

		to := &sample.Value{}
		to.SetM_1(sub2)

		diffRoundtrip(t, from, to)
	})
	t.Run("nested message deleted", func(t *testing.T) {
		sub := &sample.Value{}
		sub.SetS_1("inner")

		from := &sample.Value{}
		from.SetM_1(sub)

		diffRoundtrip(t, from, &sample.Value{})
	})
	t.Run("empty to empty", func(t *testing.T) {
		delta, err := patchproto.Diff(&sample.Value{}, &sample.Value{})
		x.NoError(t, err)
		if len(delta.GetEntries()) != 0 {
			t.Errorf("expected empty delta, got %d entries", len(delta.GetEntries()))
		}
	})
}

func TestDiffList(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar"})
		diffRoundtrip(t, a, a)
	})
	t.Run("element updated", func(t *testing.T) {
		from := &sample.Value{}
		from.SetRS_1([]string{"foo", "bar", "baz"})

		to := &sample.Value{}
		to.SetRS_1([]string{"foo", "X", "baz"})

		diffRoundtrip(t, from, to)
	})
	t.Run("elements appended", func(t *testing.T) {
		from := &sample.Value{}
		from.SetRS_1([]string{"foo"})

		to := &sample.Value{}
		to.SetRS_1([]string{"foo", "bar", "baz"})

		diffRoundtrip(t, from, to)
	})
	t.Run("elements truncated", func(t *testing.T) {
		from := &sample.Value{}
		from.SetRS_1([]string{"foo", "bar", "baz"})

		to := &sample.Value{}
		to.SetRS_1([]string{"foo"})

		diffRoundtrip(t, from, to)
	})
	t.Run("list cleared", func(t *testing.T) {
		from := &sample.Value{}
		from.SetRS_1([]string{"foo", "bar"})

		diffRoundtrip(t, from, &sample.Value{})
	})
	t.Run("list created", func(t *testing.T) {
		to := &sample.Value{}
		to.SetRS_1([]string{"foo", "bar"})

		diffRoundtrip(t, &sample.Value{}, to)
	})
	t.Run("message list diff", func(t *testing.T) {
		m0 := &sample.Value{}
		m0.SetS_1("a")
		m1 := &sample.Value{}
		m1.SetS_1("b")

		from := &sample.Value{}
		from.SetRM_1([]*sample.Value{m0, m1})

		m0b := &sample.Value{}
		m0b.SetS_1("a")
		m0b.SetS_2("extra")
		m2 := &sample.Value{}
		m2.SetS_1("c")

		to := &sample.Value{}
		to.SetRM_1([]*sample.Value{m0b, m1, m2})

		diffRoundtrip(t, from, to)
	})
}

func TestDiffMap(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"A": "a", "B": "b"})
		diffRoundtrip(t, a, a)
	})
	t.Run("value updated", func(t *testing.T) {
		from := &sample.Value{}
		from.SetMSS(map[string]string{"A": "a", "B": "b"})

		to := &sample.Value{}
		to.SetMSS(map[string]string{"A": "a", "B": "X"})

		diffRoundtrip(t, from, to)
	})
	t.Run("key added", func(t *testing.T) {
		from := &sample.Value{}
		from.SetMSS(map[string]string{"A": "a"})

		to := &sample.Value{}
		to.SetMSS(map[string]string{"A": "a", "B": "b"})

		diffRoundtrip(t, from, to)
	})
	t.Run("key deleted", func(t *testing.T) {
		from := &sample.Value{}
		from.SetMSS(map[string]string{"A": "a", "B": "b"})

		to := &sample.Value{}
		to.SetMSS(map[string]string{"A": "a"})

		diffRoundtrip(t, from, to)
	})
	t.Run("map cleared", func(t *testing.T) {
		from := &sample.Value{}
		from.SetMSS(map[string]string{"A": "a"})

		diffRoundtrip(t, from, &sample.Value{})
	})
	t.Run("map created", func(t *testing.T) {
		to := &sample.Value{}
		to.SetMSS(map[string]string{"A": "a"})

		diffRoundtrip(t, &sample.Value{}, to)
	})
	t.Run("message map diff", func(t *testing.T) {
		ma := &sample.Value{}
		ma.SetS_1("old")
		mb := &sample.Value{}
		mb.SetS_1("keep")

		from := &sample.Value{}
		from.SetMSM(map[string]*sample.Value{"A": ma, "B": mb})

		ma2 := &sample.Value{}
		ma2.SetS_1("new")
		mc := &sample.Value{}
		mc.SetS_1("added")

		to := &sample.Value{}
		to.SetMSM(map[string]*sample.Value{"A": ma2, "B": mb, "C": mc})

		diffRoundtrip(t, from, to)
	})
}
