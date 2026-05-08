package dpb_test

import (
	"slices"
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
)

func TestPathRoundtrip(t *testing.T) {
	collect := func(p dpb.Path) []any {
		return slices.Collect(p.Seq())
	}

	t.Run("string", func(t *testing.T) {
		got := collect(dpb.P.S("foo"))
		x.Eq(t, []any{"foo"}, got)
	})
	t.Run("string empty", func(t *testing.T) {
		got := collect(dpb.P.S(""))
		x.Eq(t, []any{""}, got)
	})
	t.Run("int positive", func(t *testing.T) {
		got := collect(dpb.P.I(42))
		x.Eq(t, []any{42}, got)
	})
	t.Run("int negative", func(t *testing.T) {
		got := collect(dpb.P.I(-1))
		x.Eq(t, []any{-1}, got)
	})
	t.Run("int zero", func(t *testing.T) {
		got := collect(dpb.P.I(0))
		x.Eq(t, []any{0}, got)
	})
	t.Run("uint", func(t *testing.T) {
		got := collect(dpb.P.U(100))
		x.Eq(t, []any{uint(100)}, got)
	})
	t.Run("zigzag positive", func(t *testing.T) {
		got := collect(dpb.P.Z(7))
		x.Eq(t, []any{7}, got)
	})
	t.Run("zigzag negative", func(t *testing.T) {
		got := collect(dpb.P.Z(-7))
		x.Eq(t, []any{-7}, got)
	})
	t.Run("large values", func(t *testing.T) {
		got := collect(dpb.P.U(1 << 32))
		x.Eq(t, []any{uint(1 << 32)}, got)
	})
	t.Run("multiple segments", func(t *testing.T) {
		got := collect(dpb.P.S("a", "b").I(1, -2).U(3).Z(-4, 5))
		x.Eq(t, []any{"a", "b", 1, -2, uint(3), -4, 5}, got)
	})
	t.Run("chained", func(t *testing.T) {
		got := collect(dpb.P.S("key").I(3).Z(-1))
		x.Eq(t, []any{"key", 3, -1}, got)
	})
}
