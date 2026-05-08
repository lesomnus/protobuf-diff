package patchstruct

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// mapKeyProtoKind returns the protoreflect.Kind matching the Go map key type,
// used to reuse target/ref encoding helpers.
func mapKeyProtoKind(t reflect.Type) (protoreflect.Kind, error) {
	switch t.Kind() {
	case reflect.String:
		return protoreflect.StringKind, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return protoreflect.Int64Kind, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return protoreflect.Uint64Kind, nil
	case reflect.Bool:
		return protoreflect.BoolKind, nil
	default:
		return 0, fmt.Errorf("unsupported map key type: %v", t.Kind())
	}
}

// protoKeyToReflect converts a protoreflect.MapKey to a reflect.Value of type t.
func protoKeyToReflect(k protoreflect.MapKey, t reflect.Type) reflect.Value {
	r := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.String:
		r.SetString(k.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		r.SetInt(k.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		r.SetUint(k.Uint())
	case reflect.Bool:
		r.SetBool(k.Bool())
	}
	return r
}

func (o PatchOption) patchMap(v reflect.Value, entry *dpb.Entry) error {
	kt := v.Type().Key()
	kind, err := mapKeyProtoKind(kt)
	if err != nil {
		return err
	}

	protoKeys, err := target.DecodeKeys(entry.GetTargets(), kind)
	if err != nil {
		return fmt.Errorf("decode keys: %w", err)
	}
	if len(protoKeys) == 0 {
		return nil
	}

	keys := make([]reflect.Value, len(protoKeys))
	for i, k := range protoKeys {
		keys[i] = protoKeyToReflect(k, kt)
	}

	check := func(k reflect.Value) bool {
		exists := v.MapIndex(k).IsValid()
		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	op := func(k reflect.Value) error { return nil }
	after := func() error { return nil }

	vt := v.Type().Elem()

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		}

	case dpb.Entry_Assigned_case:
		var val reflect.Value
		op = func(k reflect.Value) error {
			if !check(k) {
				return nil
			}
			if !val.IsValid() {
				bv, err := decodeValue(entry.GetAssigned(), vt)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
				val = reflect.New(vt).Elem()
				setField(val, bv)
			}
			v.SetMapIndex(k, val)
			return nil
		}

	case dpb.Entry_Merged_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	case dpb.Entry_Copied_case:
		srcKey, err := ref.DecodeKey(entry.GetCopied(), kind)
		if err != nil {
			return fmt.Errorf("copy: decode source key: %w", err)
		}
		srcRV := protoKeyToReflect(srcKey, kt)
		srcVal := v.MapIndex(srcRV)
		if !srcVal.IsValid() {
			op = func(k reflect.Value) error {
				if !check(k) {
					return nil
				}
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		} else {
			op = func(k reflect.Value) error {
				if !check(k) {
					return nil
				}
				v.SetMapIndex(k, srcVal)
				return nil
			}
		}

	case dpb.Entry_Scattered_case:
		srcKey, err := ref.DecodeKey(entry.GetScattered(), kind)
		if err != nil {
			return fmt.Errorf("scatter: decode source key: %w", err)
		}
		srcRV := protoKeyToReflect(srcKey, kt)
		srcVal := v.MapIndex(srcRV)
		if !srcVal.IsValid() {
			op = func(k reflect.Value) error {
				if !check(k) {
					return nil
				}
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		} else {
			op = func(k reflect.Value) error {
				if !check(k) {
					return nil
				}
				v.SetMapIndex(k, srcVal)
				return nil
			}
		}
		after = func() error {
			v.SetMapIndex(srcRV, reflect.Value{})
			return nil
		}

	case dpb.Entry_Swapped_case:
		srcKey, err := ref.DecodeKey(entry.GetSwapped(), kind)
		if err != nil {
			return fmt.Errorf("swap: decode source key: %w", err)
		}
		srcRV := protoKeyToReflect(srcKey, kt)
		tmp := v.MapIndex(srcRV)

		op = func(k reflect.Value) error {
			w := v.MapIndex(k)
			if tmp.IsValid() {
				v.SetMapIndex(k, tmp)
			} else {
				v.SetMapIndex(k, reflect.Value{})
			}
			tmp = w
			return nil
		}
		after = func() error {
			if tmp.IsValid() {
				v.SetMapIndex(srcRV, tmp)
			} else {
				v.SetMapIndex(srcRV, reflect.Value{})
			}
			return nil
		}

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()
		et := vt
		for et.Kind() == reflect.Pointer {
			et = et.Elem()
		}
		if et.Kind() != reflect.Struct {
			return fmt.Errorf("nested deltas for maps can only be applied to struct values, got %v", et.Kind())
		}

		op = func(k reflect.Value) error {
			mv := v.MapIndex(k)
			if !mv.IsValid() {
				return nil
			}
			// Map values are not addressable; create an addressable copy.
			ptr := reflect.New(vt)
			ptr.Elem().Set(mv)
			elem := ptr.Elem()
			for elem.Kind() == reflect.Pointer {
				if elem.IsNil() {
					return nil
				}
				elem = elem.Elem()
			}
			if err := o.PatchField(elem, delta); err != nil {
				return fmt.Errorf("key %v: %w", k, err)
			}
			v.SetMapIndex(k, ptr.Elem())
			return nil
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(keys))
	for _, k := range keys {
		if err := op(k); err != nil {
			errs = append(errs, fmt.Errorf("[%v]: %w", k, err))
		}
	}
	if err := after(); err != nil {
		errs = append(errs, fmt.Errorf("clean up: %w", err))
	}
	return errors.Join(errs...)
}
