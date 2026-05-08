package dpb

import (
	"fmt"
	"iter"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

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

func Navigate(c any, fd protoreflect.FieldDescriptor, segments []any) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	switch c := c.(type) {
	case protoreflect.Message:
		return NavigateMessage(c, fd, segments)
	case protoreflect.List:
		return NavigateList(c, fd, segments)
	case protoreflect.Map:
		return NavigateMap(c, fd, segments)
	case protoreflect.Value:
		if fd == nil {
			return nil, nil, fmt.Errorf("cannot navigate into value without field descriptor")
		}
		switch {
		case fd.IsMap():
			return NavigateMap(c.Map(), fd, segments)
		case fd.IsList():
			return NavigateList(c.List(), fd, segments)
		case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
			return NavigateMessage(c.Message(), fd, segments)
		default:
			return nil, nil, fmt.Errorf("cannot navigate into scalar value: %s", fd.Kind())
		}
	default:
		return nil, nil, fmt.Errorf("unsupported value type: %T", c)
	}
}

func NavigateMessage(c protoreflect.Message, fd protoreflect.FieldDescriptor, segments []any) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	s := segments[0]
	fields := c.Descriptor().Fields()

	var fd_ protoreflect.FieldDescriptor
	switch s := s.(type) {
	case string:
		fd_ = fields.ByName(protoreflect.Name(s))
		if fd_ == nil {
			return nil, nil, fmt.Errorf("field not found: %q", s)
		}
	case int:
		if s <= 0 {
			return nil, nil, fmt.Errorf("invalid field number: %d", s)
		}

		fd_ = fields.ByNumber(protoreflect.FieldNumber(s))
		if fd_ == nil {
			return nil, nil, fmt.Errorf("field not found: %d", s)
		}

	case uint:
		if s == 0 {
			return nil, nil, fmt.Errorf("invalid field number: %d", s)
		}

		fd_ = fields.ByNumber(protoreflect.FieldNumber(s))
		if fd_ == nil {
			return nil, nil, fmt.Errorf("field not found: %d", s)
		}

	default:
		return nil, nil, fmt.Errorf("invalid path segment type: %T", s)
	}

	v := c.Get(fd_)
	switch {
	case fd_.IsMap():
		return Navigate(v.Map(), fd_, segments[1:])
	case fd_.IsList():
		return Navigate(v.List(), fd_, segments[1:])
	case fd_.Kind() == protoreflect.MessageKind || fd_.Kind() == protoreflect.GroupKind:
		return Navigate(v.Message(), fd_, segments[1:])
	default:
		return Navigate(v, fd_, segments[1:])
	}
}

func NavigateList(c protoreflect.List, fd protoreflect.FieldDescriptor, segments []any) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	s := segments[0]
	l := c.Len()

	var v protoreflect.Value
	switch s := s.(type) {
	case int:
		if s < 0 {
			s += l
		}
		if s > l || s < 0 {
			return nil, nil, fmt.Errorf("list index out of bounds: %d", s)
		}

		v = c.Get(s)

	case uint:
		if s > uint(l) {
			return nil, nil, fmt.Errorf("list index out of bounds: %d", s)
		}

		v = c.Get(int(s))

	default:
		return nil, nil, fmt.Errorf("invalid path segment type for list: %T", s)
	}

	switch fd.Kind() {
	case protoreflect.MessageKind:
		return NavigateMessage(v.Message(), fd, segments[1:])
	default:
		return nil, nil, fmt.Errorf("cannot navigate into non-message list element: %s", fd.Kind())
	}
}

func NavigateMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, segments []any) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	s := segments[0]

	var w protoreflect.Value
	switch fd.MapKey().Kind() {
	case protoreflect.StringKind:
		var k string
		switch s := s.(type) {
		case string:
			k = s
		case int:
			k = strconv.FormatInt(int64(s), 10)
		case uint:
			k = strconv.FormatUint(uint64(s), 10)
		}
		w = protoreflect.ValueOfString(k)

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:

		var k int
		switch s := s.(type) {
		case string:
			if len(s) == 0 {
				return nil, nil, fmt.Errorf("invalid int map key: empty string")
			}
			if s[0] == '-' {
				v, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid int map key: %q", s)
				}
				k = int(v)
			} else {
				v, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid int map key: %q", s)
				}
				k = int(v)
			}
		case int:
			k = s
		case uint:
			k = int(s)

		default:
			return nil, nil, fmt.Errorf("invalid path segment type for int map key: %T", s)
		}
		w = protoreflect.ValueOfInt64(int64(k))

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:

		var k uint
		switch s := s.(type) {
		case string:
			if len(s) == 0 {
				return nil, nil, fmt.Errorf("invalid uint map key: empty string")
			}
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid uint map key: %q", s)
			}
			k = uint(v)
		case int:
			if s < 0 {
				return nil, nil, fmt.Errorf("invalid uint map key: negative int %d", s)
			}
			k = uint(s)
		case uint:
			k = s

		default:
			return nil, nil, fmt.Errorf("invalid path segment type for uint map key: %T", s)
		}
		w = protoreflect.ValueOfUint64(uint64(k))

	default:
		return nil, nil, fmt.Errorf("unsupported map key type: %s", fd.MapKey().Kind())
	}

	v := c.Get(protoreflect.MapKey(w))
	switch fd.MapValue().Kind() {
	case protoreflect.MessageKind:
		return NavigateMessage(v.Message(), fd, segments[1:])
	default:
		return nil, nil, fmt.Errorf("cannot navigate into non-message map value: %s", fd.MapValue().Kind())
	}
}
