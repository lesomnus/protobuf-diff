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

// Option configures a Patch call.
type Option func(*PatchOption)

// WithHook registers a hook that is called each time a value is modified.
// The hook receives the path of PathEntries leading to the modified value.
func WithHook(h func(pe []dpb.PathEntry, before, after dpb.Frame, entry *dpb.Entry)) Option {
	return func(o *PatchOption) {
		if o.cursor == nil {
			o.cursor = &dpb.Cursor{}
		}
		o.cursor.Hooks = append(o.cursor.Hooks, h)
	}
}

type Frame struct {
	Value any
}

func (f Frame) String() string {
	b, _ := json.Marshal(f.Value)
	return string(b)
}

func (f Frame) Apply(entry *dpb.Entry) (dpb.Frame, error) {
	switch entry.WhichKind() {
	case dpb.Entry_Assigned_case:
		v, err := decodeValue(entry.GetAssigned())
		if err != nil {
			return nil, fmt.Errorf("apply: %w", err)
		}
		return Frame{Value: v}, nil
	case dpb.Entry_Deleted_case:
		if entry.GetDeleted() {
			return Frame{}, nil
		}
		return f, nil
	default:
		return nil, fmt.Errorf("Apply: not implemented for %q", entry.WhichKind())
	}
}

// Value encodes v as JSON bytes for use in Entry.SetAssigned and similar fields.
// patchjson expects assigned bytes to be JSON-encoded values.
func Value(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func Patch(v any, delta *dpb.Delta, opts ...Option) error {
	var o PatchOption
	for _, opt := range opts {
		opt(&o)
	}
	return o.Patch(v, delta)
}

type PatchOption struct {
	// cursor is a pointer so all value-receiver copies share the same path state.
	cursor *dpb.Cursor
}

func (o PatchOption) Patch(v any, delta *dpb.Delta) error {
	o_ := o
	c := &dpb.Cursor{}
	if o.cursor != nil {
		c.Hooks = o.cursor.Hooks
	}
	o_.cursor = c

	root, rootSet, err := unwrapRoot(v)
	if err != nil {
		return err
	}
	for i, entry := range delta.GetEntries() {
		if err := o_.patch(root, rootSet, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

// cursorEnter pushes a PathEntry and returns a function that pops it.
func (o PatchOption) cursorEnter(e dpb.PathEntry) func() {
	if o.cursor == nil {
		return func() {}
	}
	o.cursor.Push(e)
	return func() { o.cursor.Pop() }
}

// cursorNotify fires all hooks with the current cursor path and entry.
func (o PatchOption) cursorNotify(before, after dpb.Frame, entry *dpb.Entry) {
	if o.cursor != nil {
		o.cursor.Notify(before, after, entry)
	}
}

func segmentToPathEntry(s any) dpb.PathEntry {
	switch s := s.(type) {
	case string:
		return dpb.PathEntry{Kind: dpb.PathEntryField, Key: s}
	case int:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: s}
	case uint:
		return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(s)}
	default:
		return dpb.PathEntry{}
	}
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

	if o.cursor != nil {
		for _, seg := range segments {
			o.cursor.Push(segmentToPathEntry(seg))
		}
		defer func() {
			for range segments {
				o.cursor.Pop()
			}
		}()
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
