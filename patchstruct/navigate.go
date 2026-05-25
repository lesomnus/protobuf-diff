package patchstruct

import (
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func Navigate(v reflect.Value, segments []*dpb.FieldSegment) (reflect.Value, error) {
	for _, fs := range segments {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, fmt.Errorf("nil pointer")
			}
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Struct:
			if fs.HasName() && fs.GetName() != "" {
				f := v.FieldByName(fs.GetName())
				if !f.IsValid() {
					return reflect.Value{}, fmt.Errorf("field %q not found", fs.GetName())
				}
				v = f
			} else {
				n := v.NumField()
				idx := int(fs.GetNumber())
				if idx < 0 {
					idx += n
				}
				if idx < 0 || idx >= n {
					return reflect.Value{}, fmt.Errorf("field index out of range: %d", fs.GetNumber())
				}
				v = v.Field(idx)
			}

		case reflect.Slice:
			l := v.Len()
			i := int(fs.GetNumber())
			if i < 0 {
				i += l
			}
			if i < 0 || i >= l {
				return reflect.Value{}, fmt.Errorf("slice index out of bounds: %d", fs.GetNumber())
			}
			v = v.Index(i)

		default:
			return reflect.Value{}, fmt.Errorf("cannot navigate into %v", v.Kind())
		}
	}
	return v, nil
}
