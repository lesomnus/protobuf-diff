package patchstruct

import (
	"fmt"
	"reflect"
)

func Navigate(v reflect.Value, segments []any) (reflect.Value, error) {
	for _, s := range segments {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, fmt.Errorf("nil pointer")
			}
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Struct:
			switch s := s.(type) {
			case string:
				f := v.FieldByName(s)
				if !f.IsValid() {
					return reflect.Value{}, fmt.Errorf("field %q not found", s)
				}
				v = f

			case int:
				n := v.NumField()
				if s < 0 {
					s += n
				}
				if s < 0 || s >= n {
					return reflect.Value{}, fmt.Errorf("field index out of range: %d", s)
				}
				v = v.Field(s)

			case uint:
				if int(s) >= v.NumField() {
					return reflect.Value{}, fmt.Errorf("field index out of range: %d", s)
				}
				v = v.Field(int(s))

			default:
				return reflect.Value{}, fmt.Errorf("invalid segment type for struct: %T", s)
			}

		case reflect.Slice:
			l := v.Len()
			var i int
			switch s := s.(type) {
			case int:
				i = s
			case uint:
				i = int(s)
			default:
				return reflect.Value{}, fmt.Errorf("invalid segment type for slice: %T", s)
			}
			if i < 0 {
				i += l
			}
			if i < 0 || i >= l {
				return reflect.Value{}, fmt.Errorf("slice index out of bounds: %d", s)
			}
			v = v.Index(i)

		default:
			return reflect.Value{}, fmt.Errorf("cannot navigate into %v", v.Kind())
		}
	}
	return v, nil
}
