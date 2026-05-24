package dpb

import (
	"iter"

	"google.golang.org/protobuf/encoding/protowire"
)

type PathEntryKind int

const (
	PathEntryField PathEntryKind = iota + 1
	PathEntryIndex
)

// PathEntry represents a human-readable segment in the path, which can be a field name or an index.
type PathEntry struct {
	Kind PathEntryKind

	Key   string
	Index int
}

// Path represents the field path to the target field, encoded as a sequence of tag and wire type pairs.
type Path struct {
	v []byte
}

// Each segment is encoded as: first byte = [continuation(1)][value_low5(5)][tag(2)], then standard 7-bit varint bytes.
// For strings, the varint is the byte length followed by the string bytes.
const (
	PathTagString byte = 0
	PathTagInt    byte = 1
	PathTagUint   byte = 2
	PathTagZigzag byte = 3
)

var P = Path{}

func (x Path) Value() []byte {
	return x.v
}

func appendSegment(b []byte, tag byte, v uint64) []byte {
	first := (byte(v&0b0001_1111) << 2) | tag
	v >>= 5
	if v > 0 {
		first |= 0b1000_0000
	}
	b = append(b, first)
	for v > 0 {
		byt := byte(v & 0b0111_1111)
		v >>= 7
		if v > 0 {
			byt |= 0b1000_0000
		}
		b = append(b, byt)
	}
	return b
}

func consumeSegment(b []byte) (tag byte, v uint64, n int) {
	if len(b) == 0 {
		return 0, 0, -1
	}
	first := b[0]
	tag = first & 0b0000_0011
	v = uint64(first>>2) & 0b0001_1111
	n = 1
	if first&0b1000_0000 == 0 {
		return
	}
	shift := uint(5)
	for n < len(b) {
		byt := b[n]
		n++
		v |= uint64(byt&0b0111_1111) << shift
		shift += 7
		if byt&0b1000_0000 == 0 {
			return
		}
	}
	return 0, 0, -1
}

// S appends the given strings to the path and returns the new path.
func (x Path) S(vs ...string) Path {
	b := append([]byte(nil), x.v...)
	for _, v := range vs {
		b = appendSegment(b, PathTagString, uint64(len(v)))
		b = append(b, v...)
	}
	return Path{b}
}

// I appends the given numbers to the path in varint encoding and returns the new path.
func (x Path) I(vs ...int) Path {
	b := append([]byte(nil), x.v...)
	for _, v := range vs {
		b = appendSegment(b, PathTagInt, uint64(v))
	}
	return Path{b}
}

// U appends the given numbers to the path in varint encoding and returns the new path.
func (x Path) U(vs ...uint) Path {
	b := append([]byte(nil), x.v...)
	for _, v := range vs {
		b = appendSegment(b, PathTagUint, uint64(v))
	}
	return Path{b}
}

// Z appends the given numbers to the path in zigzag encoding and returns the new path.
func (x Path) Z(vs ...int) Path {
	b := append([]byte(nil), x.v...)
	for _, v := range vs {
		b = appendSegment(b, PathTagZigzag, protowire.EncodeZigZag(int64(v)))
	}
	return Path{b}
}

// Seq returns the path as a sequence of strings and numbers.
// Segment type must be one of int, uint, or string.
func (x Path) Seq() iter.Seq[any] {
	return func(yield func(any) bool) {
		b := x.v
		for len(b) > 0 {
			tag, u, n := consumeSegment(b)
			if n < 0 {
				return
			}
			b = b[n:]

			var v any
			switch tag {
			case PathTagString:
				if uint64(len(b)) < u {
					return
				}
				v = string(b[:u])
				b = b[u:]
			case PathTagInt:
				v = int(u)
			case PathTagUint:
				v = uint(u)
			case PathTagZigzag:
				v = int(protowire.DecodeZigZag(u))
			}

			if !yield(v) {
				return
			}
		}
	}
}
