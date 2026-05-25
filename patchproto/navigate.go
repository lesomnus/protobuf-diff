package patchproto

import (
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Navigate(c any, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
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

func NavigateMessage(c protoreflect.Message, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	fs := segments[0]
	fields := c.Descriptor().Fields()

	var fd_ protoreflect.FieldDescriptor
	if fs.HasName() && fs.GetName() != "" {
		fd_ = fields.ByName(protoreflect.Name(fs.GetName()))
		if fd_ == nil {
			return nil, nil, fmt.Errorf("field not found: %q", fs.GetName())
		}
	} else {
		n := fs.GetNumber()
		if n <= 0 {
			return nil, nil, fmt.Errorf("invalid field number: %d", n)
		}
		fd_ = fields.ByNumber(protoreflect.FieldNumber(n))
		if fd_ == nil {
			return nil, nil, fmt.Errorf("field not found: %d", n)
		}
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

func NavigateList(c protoreflect.List, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	fs := segments[0]
	if fs.HasName() && fs.GetName() != "" {
		return nil, nil, fmt.Errorf("expected numeric index for list, got name %q", fs.GetName())
	}
	l := c.Len()
	i := int(fs.GetNumber())
	if i < 0 {
		i += l
	}
	if i < 0 || i >= l {
		return nil, nil, fmt.Errorf("list index out of bounds: %d", fs.GetNumber())
	}

	v := c.Get(i)
	switch fd.Kind() {
	case protoreflect.MessageKind:
		return NavigateMessage(v.Message(), fd, segments[1:])
	default:
		return nil, nil, fmt.Errorf("cannot navigate into non-message list element: %s", fd.Kind())
	}
}

func NavigateMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	fs := segments[0]

	var w protoreflect.Value
	switch fd.MapKey().Kind() {
	case protoreflect.StringKind:
		w = protoreflect.ValueOfString(fs.GetName())

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		w = protoreflect.ValueOfInt64(fs.GetNumber())

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		w = protoreflect.ValueOfUint64(uint64(fs.GetNumber()))

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
