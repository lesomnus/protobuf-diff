package patchstruct

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func decodeMapTargets(v reflect.Value, targets []*dpb.Segment) []reflect.Value {
	kt := v.Type().Key()
	var keys []reflect.Value
	for _, seg := range targets {
		k := reflect.New(kt).Elem()
		switch kt.Kind() {
		case reflect.String:
			if seg.WhichKind() != dpb.Segment_Name_case {
				continue
			}
			k.SetString(seg.GetName())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if seg.WhichKind() != dpb.Segment_Index_case {
				continue
			}
			k.SetInt(seg.GetIndex())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if seg.WhichKind() != dpb.Segment_Index_case {
				continue
			}
			k.SetUint(uint64(seg.GetIndex()))
		case reflect.Bool:
			if seg.WhichKind() != dpb.Segment_Name_case {
				continue
			}
			k.SetBool(seg.GetName() == "true")
		default:
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func mapSourceKey(kt reflect.Type, fs *dpb.FieldSegment) (reflect.Value, bool) {
	if fs == nil {
		return reflect.Value{}, false
	}
	k := reflect.New(kt).Elem()
	switch kt.Kind() {
	case reflect.String:
		k.SetString(fs.GetName())
		return k, true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		k.SetInt(fs.GetNumber())
		return k, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		k.SetUint(uint64(fs.GetNumber()))
		return k, true
	}
	return reflect.Value{}, false
}

func (o PatchOption) patchMap(v reflect.Value, entry *dpb.Entry) error {
	keys := decodeMapTargets(v, entry.GetTargets())
	if len(keys) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return fmt.Errorf("nest on map requires targets")
		}
		return nil
	}

	kt := v.Type().Key()
	vt := v.Type().Elem()

	op := func(k reflect.Value) error { return nil }
	after := func() error { return nil }

	switch entry.WhichKind() {
	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		}

	case dpb.Entry_Test_case:
		var val reflect.Value
		op = func(k reflect.Value) error {
			mv := v.MapIndex(k)
			if !mv.IsValid() {
				return fmt.Errorf("test failed at key %v: absent", k)
			}
			if !val.IsValid() {
				var err error
				val, err = decodeValue(entry.GetTest(), vt)
				if err != nil {
					return fmt.Errorf("decode test value: %w", err)
				}
			}
			if mv.Interface() != val.Interface() {
				return fmt.Errorf("test failed at key %v", k)
			}
			return nil
		}

	case dpb.Entry_Insert_case:
		var val reflect.Value
		op = func(k reflect.Value) error {
			if v.MapIndex(k).IsValid() {
				return nil
			}
			if !val.IsValid() {
				bv, err := decodeValue(entry.GetInsert(), vt)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
				val = reflect.New(vt).Elem()
				setField(val, bv)
			}
			v.SetMapIndex(k, val)
			return nil
		}

	case dpb.Entry_Assign_case:
		var val reflect.Value
		op = func(k reflect.Value) error {
			if !val.IsValid() {
				bv, err := decodeValue(entry.GetAssign(), vt)
				if err != nil {
					return fmt.Errorf("decode: %w", err)
				}
				val = reflect.New(vt).Elem()
				setField(val, bv)
			}
			v.SetMapIndex(k, val)
			return nil
		}

	case dpb.Entry_Move_case:
		srcRV, ok := mapSourceKey(kt, entry.GetMove())
		if !ok {
			return fmt.Errorf("move: cannot decode source key")
		}
		srcVal := v.MapIndex(srcRV)
		if !srcVal.IsValid() {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		} else {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, srcVal)
				return nil
			}
			after = func() error {
				v.SetMapIndex(srcRV, reflect.Value{})
				return nil
			}
		}

	case dpb.Entry_Copy_case:
		srcRV, ok := mapSourceKey(kt, entry.GetCopy())
		if !ok {
			return fmt.Errorf("copy: cannot decode source key")
		}
		srcVal := v.MapIndex(srcRV)
		if !srcVal.IsValid() {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, reflect.Value{})
				return nil
			}
		} else {
			op = func(k reflect.Value) error {
				v.SetMapIndex(k, srcVal)
				return nil
			}
		}

	case dpb.Entry_Nest_case:
		delta := entry.GetNest()
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
