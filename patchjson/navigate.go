package patchjson

import "fmt"

func navigate(root any, rootSet func(any), segments []any) (any, func(any), error) {
	if len(segments) == 0 {
		return root, rootSet, nil
	}

	s := segments[0]

	switch c := root.(type) {
	case map[string]any:
		key, ok := s.(string)
		if !ok {
			return nil, nil, fmt.Errorf("expected string segment for object, got %T", s)
		}
		child, exists := c[key]
		if !exists {
			return nil, nil, fmt.Errorf("key %q not found", key)
		}
		childSet := func(v any) { c[key] = v }
		return navigate(child, childSet, segments[1:])

	case []any:
		i, err := toListIndex(s, len(c))
		if err != nil {
			return nil, nil, err
		}
		child := c[i]
		childSet := func(v any) {
			c[i] = v
			rootSet(c)
		}
		return navigate(child, childSet, segments[1:])

	default:
		return nil, nil, fmt.Errorf("cannot navigate into %T", root)
	}
}

func toListIndex(s any, l int) (int, error) {
	var i int
	switch v := s.(type) {
	case int:
		i = v
	case uint:
		i = int(v)
	default:
		return 0, fmt.Errorf("expected integer segment for array, got %T", s)
	}
	if i < 0 {
		i += l
	}
	if i < 0 || i >= l {
		return 0, fmt.Errorf("array index out of bounds: %d", s)
	}
	return i, nil
}
