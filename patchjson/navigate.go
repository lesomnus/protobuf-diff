package patchjson

import (
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func navigate(root any, rootSet func(any), segments []*dpb.FieldSegment) (any, func(any), error) {
	if len(segments) == 0 {
		return root, rootSet, nil
	}

	fs := segments[0]

	switch c := root.(type) {
	case map[string]any:
		key := fs.GetName()
		child, exists := c[key]
		if !exists {
			return nil, nil, fmt.Errorf("key %q not found", key)
		}
		childSet := func(v any) { c[key] = v }
		return navigate(child, childSet, segments[1:])

	case []any:
		i, err := toListIndex(int(fs.GetNumber()), len(c))
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

func toListIndex(i, l int) (int, error) {
	if i < 0 {
		i += l
	}
	if i < 0 || i >= l {
		return 0, fmt.Errorf("array index out of bounds: %d", i)
	}
	return i, nil
}
