package patchproto

import (
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Navigate resolves a path of FieldSegments to the container (message, list, or
// map) it addresses, read-only. Use it when no mutation follows.
func Navigate(c any, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	return navigate(c, fd, segments, false)
}

// navigate is the mutation-aware implementation of Navigate. When mutate is
// true, descending into an unset message/list/map field returns an error rather
// than yielding an immutable zero container that would panic when mutated.
func navigate(c any, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment, mutate bool) (any, protoreflect.FieldDescriptor, error) {
	if len(segments) == 0 {
		return c, fd, nil
	}

	switch c := c.(type) {
	case protoreflect.Message:
		return navigateMessage(c, fd, segments, mutate)
	case protoreflect.List:
		return navigateList(c, fd, segments, mutate)
	case protoreflect.Map:
		return navigateMap(c, fd, segments, mutate)
	case protoreflect.Value:
		if fd == nil {
			return nil, nil, fmt.Errorf("cannot navigate into value without field descriptor")
		}
		switch {
		case fd.IsMap():
			return navigateMap(c.Map(), fd, segments, mutate)
		case fd.IsList():
			return navigateList(c.List(), fd, segments, mutate)
		case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
			return navigateMessage(c.Message(), fd, segments, mutate)
		default:
			return nil, nil, fmt.Errorf("cannot navigate into scalar value: %s", fd.Kind())
		}
	default:
		return nil, nil, fmt.Errorf("unsupported value type: %T", c)
	}
}

// NavigateMessage resolves a path starting from a message, read-only.
func NavigateMessage(c protoreflect.Message, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	return navigateMessage(c, fd, segments, false)
}

func navigateMessage(c protoreflect.Message, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment, mutate bool) (any, protoreflect.FieldDescriptor, error) {
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

	// A mutating op cannot descend into an unset message/list/map field: Get
	// would return an immutable zero container that panics on mutation. Error
	// cleanly instead. (Reads are fine — an immutable empty container is valid.)
	isContainer := fd_.IsMap() || fd_.IsList() ||
		fd_.Kind() == protoreflect.MessageKind || fd_.Kind() == protoreflect.GroupKind
	if mutate && isContainer && !c.Has(fd_) {
		return nil, nil, fmt.Errorf("cannot navigate into unset field %q for a mutating operation", fd_.Name())
	}

	v := c.Get(fd_)
	switch {
	case fd_.IsMap():
		return navigate(v.Map(), fd_, segments[1:], mutate)
	case fd_.IsList():
		return navigate(v.List(), fd_, segments[1:], mutate)
	case fd_.Kind() == protoreflect.MessageKind || fd_.Kind() == protoreflect.GroupKind:
		return navigate(v.Message(), fd_, segments[1:], mutate)
	default:
		return navigate(v, fd_, segments[1:], mutate)
	}
}

// NavigateList resolves a path starting from a list, read-only.
func NavigateList(c protoreflect.List, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	return navigateList(c, fd, segments, false)
}

func navigateList(c protoreflect.List, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment, mutate bool) (any, protoreflect.FieldDescriptor, error) {
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
		return navigateMessage(v.Message(), fd, segments[1:], mutate)
	default:
		return nil, nil, fmt.Errorf("cannot navigate into non-message list element: %s", fd.Kind())
	}
}

// NavigateMap resolves a path starting from a map, read-only.
func NavigateMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment) (any, protoreflect.FieldDescriptor, error) {
	return navigateMap(c, fd, segments, false)
}

func navigateMap(c protoreflect.Map, fd protoreflect.FieldDescriptor, segments []*dpb.FieldSegment, mutate bool) (any, protoreflect.FieldDescriptor, error) {
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

	key := protoreflect.MapKey(w)
	v := c.Get(key)
	switch fd.MapValue().Kind() {
	case protoreflect.MessageKind:
		// An absent key yields an invalid value; navigating into it (whether to
		// read or mutate) would panic, so error cleanly for both.
		if !v.IsValid() {
			return nil, nil, fmt.Errorf("cannot navigate into unset map key %v", key)
		}
		return navigateMessage(v.Message(), fd, segments[1:], mutate)
	default:
		return nil, nil, fmt.Errorf("cannot navigate into non-message map value: %s", fd.MapValue().Kind())
	}
}
