package dpb_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/x"
)

func ptr[T any](v T) *T { return &v }

func entryField(key string) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryField, Key: key}
}

func entryIndex(idx int) dpb.PathEntry {
	return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: idx}
}

func TestPathMatch(t *testing.T) {
	// pseg builds a pattern FieldSegment matched by name.
	pseg := func(name string) *dpb.FieldSegment {
		return dpb.FieldSegment_builder{Name: ptr(name)}.Build()
	}
	// pnum builds a pattern FieldSegment matched by number.
	pnum := func(n int64) *dpb.FieldSegment {
		return dpb.FieldSegment_builder{Number: ptr(n)}.Build()
	}
	pat := func(segs ...*dpb.FieldSegment) *dpb.Path {
		return dpb.Path_builder{Segments: segs}.Build()
	}

	t.Run("empty pattern matches empty path", func(t *testing.T) {
		x.Eq(t, true, pat().Match(nil))
	})
	t.Run("empty pattern does not match non-empty path", func(t *testing.T) {
		x.Eq(t, false, pat().Match([]dpb.PathEntry{entryField("a")}))
	})
	t.Run("single segment exact match", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("a")).Match([]dpb.PathEntry{entryField("a")}))
		x.Eq(t, false, pat(pseg("a")).Match([]dpb.PathEntry{entryField("b")}))
	})
	t.Run("multi-segment match", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("a"), pseg("b"), pseg("c")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryField("c")}))
		x.Eq(t, false, pat(pseg("a"), pseg("b")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryField("c")}))
		x.Eq(t, false, pat(pseg("a"), pseg("b"), pseg("c")).Match([]dpb.PathEntry{entryField("a"), entryField("b")}))
	})
	t.Run("single * glob matches one segment", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("fo*")).Match([]dpb.PathEntry{entryField("foo")}))
		x.Eq(t, false, pat(pseg("fo*")).Match([]dpb.PathEntry{entryField("bar")}))
	})
	t.Run("** matches zero segments", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("**")).Match(nil))
	})
	t.Run("** matches one segment", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("**")).Match([]dpb.PathEntry{entryField("a")}))
	})
	t.Run("** matches multiple segments", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("**")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryIndex(3)}))
	})
	t.Run("** at start", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("**"), pseg("c")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryField("c")}))
		x.Eq(t, true, pat(pseg("**"), pseg("c")).Match([]dpb.PathEntry{entryField("c")}))
		x.Eq(t, false, pat(pseg("**"), pseg("c")).Match([]dpb.PathEntry{entryField("a"), entryField("b")}))
	})
	t.Run("** at end", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("a"), pseg("**")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryField("c")}))
		x.Eq(t, true, pat(pseg("a"), pseg("**")).Match([]dpb.PathEntry{entryField("a")}))
		x.Eq(t, false, pat(pseg("a"), pseg("**")).Match([]dpb.PathEntry{entryField("b")}))
	})
	t.Run("** in middle", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("a"), pseg("**"), pseg("z")).Match([]dpb.PathEntry{entryField("a"), entryField("x"), entryField("y"), entryField("z")}))
		x.Eq(t, true, pat(pseg("a"), pseg("**"), pseg("z")).Match([]dpb.PathEntry{entryField("a"), entryField("z")}))
		x.Eq(t, false, pat(pseg("a"), pseg("**"), pseg("z")).Match([]dpb.PathEntry{entryField("a"), entryField("x")}))
	})
	t.Run("multiple **", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("**"), pseg("b"), pseg("**")).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryField("c"), entryField("d")}))
		x.Eq(t, true, pat(pseg("**"), pseg("b"), pseg("**")).Match([]dpb.PathEntry{entryField("b")}))
		x.Eq(t, false, pat(pseg("**"), pseg("b"), pseg("**")).Match([]dpb.PathEntry{entryField("a"), entryField("c")}))
	})
	t.Run("** with index entries", func(t *testing.T) {
		x.Eq(t, true, pat(pseg("a"), pseg("**"), pnum(3)).Match([]dpb.PathEntry{entryField("a"), entryField("b"), entryIndex(3)}))
	})
}

func TestSegmentMatch(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		t.Run("exact match", func(t *testing.T) {
			seg := dpb.Segment_builder{Name: ptr("foo")}.Build()
			x.Eq(t, true, seg.Match(entryField("foo")))
		})
		t.Run("no match", func(t *testing.T) {
			seg := dpb.Segment_builder{Name: ptr("foo")}.Build()
			x.Eq(t, false, seg.Match(entryField("bar")))
		})
		t.Run("glob star", func(t *testing.T) {
			seg := dpb.Segment_builder{Name: ptr("foo*")}.Build()
			x.Eq(t, true, seg.Match(entryField("foobar")))
			x.Eq(t, true, seg.Match(entryField("foo")))
			x.Eq(t, false, seg.Match(entryField("bar")))
		})
		t.Run("glob question", func(t *testing.T) {
			seg := dpb.Segment_builder{Name: ptr("fo?")}.Build()
			x.Eq(t, true, seg.Match(entryField("foo")))
			x.Eq(t, false, seg.Match(entryField("fooo")))
		})
		t.Run("index entry does not match", func(t *testing.T) {
			seg := dpb.Segment_builder{Name: ptr("*")}.Build()
			x.Eq(t, false, seg.Match(entryIndex(0)))
		})
	})
	t.Run("index", func(t *testing.T) {
		t.Run("match", func(t *testing.T) {
			seg := dpb.Segment_builder{Index: ptr(int64(3))}.Build()
			x.Eq(t, true, seg.Match(entryIndex(3)))
		})
		t.Run("no match", func(t *testing.T) {
			seg := dpb.Segment_builder{Index: ptr(int64(3))}.Build()
			x.Eq(t, false, seg.Match(entryIndex(5)))
		})
		t.Run("field entry does not match", func(t *testing.T) {
			seg := dpb.Segment_builder{Index: ptr(int64(3))}.Build()
			x.Eq(t, false, seg.Match(entryField("foo")))
		})
	})
	t.Run("field", func(t *testing.T) {
		t.Run("delegates to FieldSegment.Match", func(t *testing.T) {
			fs := dpb.FieldSegment_builder{Name: ptr("foo")}.Build()
			seg := dpb.Segment_builder{Field: fs}.Build()
			x.Eq(t, true, seg.Match(entryField("foo")))
			x.Eq(t, false, seg.Match(entryField("bar")))
		})
	})
	t.Run("range", func(t *testing.T) {
		t.Run("delegates to RangeSegment.Match", func(t *testing.T) {
			rs := dpb.RangeSegment_builder{Begin: ptr(int64(1)), End: ptr(int64(3))}.Build()
			seg := dpb.Segment_builder{Range: rs}.Build()
			x.Eq(t, true, seg.Match(entryIndex(1)))
			x.Eq(t, false, seg.Match(entryIndex(3)))
		})
	})
	t.Run("not set returns false", func(t *testing.T) {
		seg := dpb.Segment_builder{}.Build()
		x.Eq(t, false, seg.Match(entryField("foo")))
	})
}

func TestFieldSegmentMatch(t *testing.T) {
	t.Run("name only", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{Name: ptr("foo")}.Build()
		x.Eq(t, true, x_.Match(entryField("foo")))
		x.Eq(t, false, x_.Match(entryField("bar")))
	})
	t.Run("name glob", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{Name: ptr("foo*")}.Build()
		x.Eq(t, true, x_.Match(entryField("foobar")))
		x.Eq(t, false, x_.Match(entryField("bar")))
	})
	t.Run("name requires field entry", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{Name: ptr("*")}.Build()
		x.Eq(t, false, x_.Match(entryIndex(0)))
	})
	t.Run("name_alt matches key", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{NameAlt: ptr("fooBar")}.Build()
		x.Eq(t, true, x_.Match(entryField("fooBar")))
		x.Eq(t, false, x_.Match(entryField("barBaz")))
	})
	t.Run("name_alt glob", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{NameAlt: ptr("foo*")}.Build()
		x.Eq(t, true, x_.Match(entryField("fooBar")))
		x.Eq(t, false, x_.Match(entryField("barBaz")))
	})
	t.Run("number only", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{Number: ptr(int64(3))}.Build()
		x.Eq(t, true, x_.Match(entryIndex(3)))
		x.Eq(t, false, x_.Match(entryIndex(5)))
	})
	t.Run("number requires index entry", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{Number: ptr(int64(3))}.Build()
		x.Eq(t, false, x_.Match(entryField("foo")))
	})
	t.Run("name and number both must match", func(t *testing.T) {
		// contradictory: name requires field entry, number requires index entry → always false
		x_ := dpb.FieldSegment_builder{Name: ptr("foo"), Number: ptr(int64(3))}.Build()
		x.Eq(t, false, x_.Match(entryField("foo")))
		x.Eq(t, false, x_.Match(entryIndex(3)))
	})
	t.Run("empty x matches any entry", func(t *testing.T) {
		x_ := dpb.FieldSegment_builder{}.Build()
		x.Eq(t, true, x_.Match(entryField("anything")))
		x.Eq(t, true, x_.Match(entryIndex(99)))
	})
}

func TestRangeSegmentMatch(t *testing.T) {
	t.Run("field entry returns false", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{}.Build()
		x.Eq(t, false, rs.Match(entryField("foo")))
	})
	t.Run("open range matches all indices", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{}.Build()
		x.Eq(t, true, rs.Match(entryIndex(0)))
		x.Eq(t, true, rs.Match(entryIndex(100)))
	})
	t.Run("open end: begin to infinity", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{Begin: ptr(int64(2))}.Build()
		x.Eq(t, false, rs.Match(entryIndex(0)))
		x.Eq(t, false, rs.Match(entryIndex(1)))
		x.Eq(t, true, rs.Match(entryIndex(2)))
		x.Eq(t, true, rs.Match(entryIndex(99)))
	})
	t.Run("closed range [begin, end)", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{Begin: ptr(int64(1)), End: ptr(int64(4))}.Build()
		x.Eq(t, false, rs.Match(entryIndex(0)))
		x.Eq(t, true, rs.Match(entryIndex(1)))
		x.Eq(t, true, rs.Match(entryIndex(3)))
		x.Eq(t, false, rs.Match(entryIndex(4)))
	})
	t.Run("open begin defaults to 0", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{End: ptr(int64(3))}.Build()
		x.Eq(t, true, rs.Match(entryIndex(0)))
		x.Eq(t, true, rs.Match(entryIndex(2)))
		x.Eq(t, false, rs.Match(entryIndex(3)))
	})
	t.Run("negative begin returns false", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{Begin: ptr(int64(-2))}.Build()
		x.Eq(t, false, rs.Match(entryIndex(0)))
		x.Eq(t, false, rs.Match(entryIndex(5)))
	})
	t.Run("negative end returns false", func(t *testing.T) {
		rs := dpb.RangeSegment_builder{End: ptr(int64(-1))}.Build()
		x.Eq(t, false, rs.Match(entryIndex(0)))
	})
}
