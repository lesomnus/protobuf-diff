package patchstruct

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/target"
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
	segments := slices.Collect(entry.Path().Seq())

	var s any
	if len(entry.GetTargets()) == 0 {
		if len(segments) == 0 {
			return fmt.Errorf("empty path and no targets")
		}

		segments, s = segments[:len(segments)-1], segments[len(segments)-1]
	}

	v, err := Navigate(v, segments)
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
		if s != nil {
			switch s := s.(type) {
			case string:
				entry.AppendTargets(target.StringKeys(s))

			case int:
				n := v.NumField()
				if s < 0 {
					s += n
				}
				if s < 0 || s >= n {
					return fmt.Errorf("field index out of range: %d", s)
				}
				entry.AppendTargets(target.StringKeys(v.Type().Field(s).Name))

			case uint:
				if int(s) >= v.NumField() {
					return fmt.Errorf("field index out of range: %d", s)
				}
				entry.AppendTargets(target.StringKeys(v.Type().Field(int(s)).Name))

			default:
				return fmt.Errorf("invalid target segment type for struct: %T", s)
			}
		}
		return o.patchStruct(v, entry)

	case reflect.Slice:
		if s != nil {
			i := 0
			switch s := s.(type) {
			case int:
				i = s
			case uint:
				i = int(s)
			default:
				return fmt.Errorf("invalid target segment type for slice: %T", s)
			}

			entry.AppendTargets(target.Indices(i))
		}
		return o.patchSlice(v, entry)

	case reflect.Map:
		if s != nil {
			var k target.Targets
			switch s := s.(type) {
			case string:
				k = target.StringKeys(s)
			case int:
				k = target.SignedKeys(s)
			case uint:
				k = target.UnsignedKeys(s)
			default:
				return fmt.Errorf("invalid target segment type for map: %T", s)
			}

			entry.AppendTargets(k)
		}
		return o.patchMap(v, entry)

	default:
		return fmt.Errorf("unsupported container type: %v", v.Kind())
	}
}
