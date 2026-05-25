package patchjson

import (
	"encoding/json"
	"fmt"

	"github.com/lesomnus/protobuf-diff/dpb"
)

type Patcher interface {
	Patch(v any, delta *dpb.Delta) error
}

// Option configures a Patch call.
type Option func(*PatchOption)

// WithHook registers a hook that is called each time a value is modified.
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

// Value creates a *dpb.Value from any JSON-compatible Go value.
func Value(v any) *dpb.Value {
	return anyToValue(v)
}

func anyToValue(v any) *dpb.Value {
	if v == nil {
		return dpb.ValNull()
	}
	switch t := v.(type) {
	case string:
		return dpb.ValS(t)
	case bool:
		return dpb.ValB(t)
	case float64:
		return dpb.ValF(t)
	case int:
		return dpb.ValI(int64(t))
	case int64:
		return dpb.ValI(t)
	case []byte:
		return dpb.ValX(t)
	default:
		return dpb.ValNull()
	}
}

func dpbValueToAny(v *dpb.Value) any {
	if v == nil {
		return nil
	}
	switch v.WhichKind() {
	case dpb.Value_N_case:
		return nil
	case dpb.Value_S_case:
		return v.GetS()
	case dpb.Value_B_case:
		return v.GetB()
	case dpb.Value_I_case:
		return float64(v.GetI())
	case dpb.Value_U_case:
		return float64(v.GetU())
	case dpb.Value_F_case:
		return v.GetF()
	case dpb.Value_X_case:
		return v.GetX()
	default:
		return nil
	}
}

func Patch(v any, delta *dpb.Delta, opts ...Option) error {
	var o PatchOption
	for _, opt := range opts {
		opt(&o)
	}
	return o.Patch(v, delta)
}

type PatchOption struct {
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

func (o PatchOption) cursorEnter(e dpb.PathEntry) func() {
	if o.cursor == nil {
		return func() {}
	}
	o.cursor.Push(e)
	return func() { o.cursor.Pop() }
}

func (o PatchOption) cursorNotify(before, after dpb.Frame, entry *dpb.Entry) {
	if o.cursor != nil {
		o.cursor.Notify(before, after, entry)
	}
}

func fieldSegToPathEntry(fs *dpb.FieldSegment) dpb.PathEntry {
	if fs.HasName() && fs.GetName() != "" {
		return dpb.PathEntry{Kind: dpb.PathEntryField, Key: fs.GetName()}
	}
	return dpb.PathEntry{Kind: dpb.PathEntryIndex, Index: int(fs.GetNumber())}
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
	pathSegs := entry.GetPath().GetSegments()

	if o.cursor != nil {
		for _, fs := range pathSegs {
			o.cursor.Push(fieldSegToPathEntry(fs))
		}
		defer func() {
			for range pathSegs {
				o.cursor.Pop()
			}
		}()
	}

	container, containerSet, err := navigate(root, rootSet, pathSegs)
	if err != nil {
		return fmt.Errorf("navigate path: %w", err)
	}

	switch c := container.(type) {
	case map[string]any:
		return o.patchMap(c, entry)
	case []any:
		return o.patchList(c, containerSet, entry)
	default:
		return fmt.Errorf("cannot patch %T", container)
	}
}

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
