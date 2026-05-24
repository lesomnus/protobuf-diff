package patchjson

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/target"
)

type Patcher interface {
	Patch(v any, delta *dpb.Delta) error
}

// Value encodes v as JSON bytes for use in Entry.SetAssigned and similar fields.
// patchjson expects assigned bytes to be JSON-encoded values.
func Value(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func Patch(v any, delta *dpb.Delta) error {
	return PatchOption{}.Patch(v, delta)
}

type PatchOption struct{}

func (o PatchOption) Patch(v any, delta *dpb.Delta) error {
	root, rootSet, err := unwrapRoot(v)
	if err != nil {
		return err
	}
	for i, entry := range delta.GetEntries() {
		if err := o.patch(root, rootSet, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

func unwrapRoot(v any) (any, func(any), error) {
	switch c := v.(type) {
	case map[string]any:
		return c, func(any) {}, nil
	case *[]any:
		return *c, func(nv any) { *c = nv.([]any) }, nil
	case *any:
		switch d := (*c).(type) {
		case map[string]any:
			return d, func(nv any) { *c = nv }, nil
		case []any:
			return d, func(nv any) { *c = nv }, nil
		}
	}

	return nil, nil, fmt.Errorf("value must be map[string]any or *[]any, got %T", v)
}

func (o PatchOption) patch(root any, rootSet func(any), entry *dpb.Entry) error {
	segments := slices.Collect(entry.Path().Seq())

	var s any
	if len(entry.GetTargets()) == 0 {
		if len(segments) == 0 {
			return fmt.Errorf("empty path and no targets")
		}
		segments, s = segments[:len(segments)-1], segments[len(segments)-1]
	}

	container, containerSet, err := navigate(root, rootSet, segments)
	if err != nil {
		return fmt.Errorf("navigate path: %w", err)
	}

	switch c := container.(type) {
	case map[string]any:
		if s != nil {
			key, ok := s.(string)
			if !ok {
				return fmt.Errorf("invalid target segment for object: %T", s)
			}
			entry.AppendTargets(target.StringKeys(key))
		}
		return o.patchMap(c, entry)

	case []any:
		if s != nil {
			switch sv := s.(type) {
			case int:
				entry.AppendTargets(target.Indices(sv))
			case uint:
				entry.AppendTargets(target.Indices(int(sv)))
			default:
				return fmt.Errorf("invalid target segment for array: %T", s)
			}
		}
		return o.patchList(c, containerSet, entry)

	default:
		return fmt.Errorf("cannot patch %T", container)
	}
}

// patchField applies a nested delta to a child container, propagating slice write-backs via set.
func (o PatchOption) patchField(v any, set func(any), delta *dpb.Delta) error {
	switch c := v.(type) {
	case map[string]any:
		for i, entry := range delta.GetEntries() {
			if err := o.patch(c, set, entry); err != nil {
				return fmt.Errorf("entry[%d]: %w", i, err)
			}
		}
		return nil
	case []any:
		cur := c
		curSet := func(nv any) {
			cur = nv.([]any)
			set(nv)
		}
		for i, entry := range delta.GetEntries() {
			if err := o.patch(cur, curSet, entry); err != nil {
				return fmt.Errorf("entry[%d]: %w", i, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("nested delta cannot be applied to %T", v)
	}
}

func decodeValue(b []byte) (any, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}
