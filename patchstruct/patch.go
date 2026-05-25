package patchstruct

import (
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
)

type Patcher interface {
	Patch(v any, delta *dpb.Delta) error
}

func Patch(v any, delta *dpb.Delta) error {
	return PatchOption{}.Patch(v, delta)
}

type PatchOption struct{}

func (o PatchOption) Patch(v any, delta *dpb.Delta) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("value must be a non-nil pointer to a struct")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("value must be a pointer to a struct, got pointer to %v", rv.Kind())
	}
	return o.PatchField(rv, delta)
}

func (o PatchOption) PatchField(v reflect.Value, delta *dpb.Delta) error {
	for i, entry := range delta.GetEntries() {
		if err := o.patch(v, entry); err != nil {
			return fmt.Errorf("entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (o PatchOption) patch(v reflect.Value, entry *dpb.Entry) error {
	pathSegs := entry.GetPath().GetSegments()

	v, err := Navigate(v, pathSegs)
	if err != nil {
		return fmt.Errorf("navigate path: %w", err)
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fmt.Errorf("nil pointer at end of path")
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		return o.patchStruct(v, entry)
	case reflect.Slice:
		return o.patchSlice(v, entry)
	case reflect.Map:
		return o.patchMap(v, entry)
	default:
		return fmt.Errorf("unsupported container type: %v", v.Kind())
	}
}
