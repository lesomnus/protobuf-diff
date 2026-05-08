package patchproto

import (
	"fmt"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

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
