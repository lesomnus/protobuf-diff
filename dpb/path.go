package dpb

import "path"

type PathEntryKind int

const (
	PathEntryField PathEntryKind = iota + 1
	PathEntryIndex
)

// PathEntry represents a human-readable segment in the path for the cursor.
type PathEntry struct {
	Kind  PathEntryKind
	Key   string
	Index int
}

// Match reports whether ys matches the pattern path x.
// Each segment in x is matched one-to-one against ys using FieldSegment.Match,
// except when a segment has name "**", which matches zero or more segments in ys.
func (x *Path) Match(ys []PathEntry) bool {
	return matchPath(x.GetSegments(), ys)
}

func matchPath(xs []*FieldSegment, ys []PathEntry) bool {
	for len(xs) > 0 {
		head := xs[0]
		xs = xs[1:]

		if head.HasName() && head.GetName() == "**" {
			for i := 0; i <= len(ys); i++ {
				if matchPath(xs, ys[i:]) {
					return true
				}
			}
			return false
		}

		if len(ys) == 0 {
			return false
		}
		if !head.Match(ys[0]) {
			return false
		}
		ys = ys[1:]
	}
	return len(ys) == 0
}

func (x *Segment) Match(y PathEntry) bool {
	switch x.WhichKind() {
	case Segment_Name_case:
		if y.Kind != PathEntryField {
			return false
		}
		ok, _ := path.Match(x.GetName(), y.Key)
		return ok
	case Segment_Index_case:
		if y.Kind != PathEntryIndex {
			return false
		}
		return x.GetIndex() == int64(y.Index)
	case Segment_Field_case:
		return x.GetField().Match(y)
	case Segment_Range_case:
		return x.GetRange().Match(y)
	default:
		return false
	}
}

// Match reports whether y satisfies all conditions specified in x.
// Each set field in x is ANDed; name and name_alt both match against y.Key, number matches y.Index.
// Name and name_alt patterns may contain glob wildcards (*/?/[]).
func (x *FieldSegment) Match(y PathEntry) bool {
	if x.HasName() {
		if y.Kind != PathEntryField {
			return false
		}
		ok, _ := path.Match(x.GetName(), y.Key)
		if !ok {
			return false
		}
	}
	if x.HasNameAlt() {
		if y.Kind != PathEntryField {
			return false
		}
		ok, _ := path.Match(x.GetNameAlt(), y.Key)
		if !ok {
			return false
		}
	}
	if x.HasNumber() {
		if y.Kind != PathEntryIndex {
			return false
		}
		if x.GetNumber() != int64(y.Index) {
			return false
		}
	}
	return true
}

// Match reports whether y.Index falls within the range [begin, end).
// Negative begin/end require knowing the collection size and are not supported here; they return false.
func (x *RangeSegment) Match(y PathEntry) bool {
	if y.Kind != PathEntryIndex {
		return false
	}

	idx := int64(y.Index)

	begin := int64(0)
	if x.HasBegin() {
		b := x.GetBegin()
		if b < 0 {
			return false
		}
		begin = b
	}

	if idx < begin {
		return false
	}

	if x.HasEnd() {
		e := x.GetEnd()
		if e < 0 {
			return false
		}
		if idx >= e {
			return false
		}
	}

	return true
}
