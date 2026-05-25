package patchstruct

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
)

func decodeStructTargets(targets []*dpb.Segment) []string {
	var names []string
	for _, seg := range targets {
		if seg.WhichKind() == dpb.Segment_Name_case {
			names = append(names, seg.GetName())
		}
	}
	return names
}

func (o PatchOption) patchStruct(v reflect.Value, entry *dpb.Entry) error {
	names := decodeStructTargets(entry.GetTargets())
	if len(names) == 0 {
		if entry.WhichKind() == dpb.Entry_Nest_case {
			return o.PatchField(v, entry.GetNest())
		}
		return nil
	}

	op := func(name string) error { return nil }

	switch entry.WhichKind() {
	case dpb.Entry_Remove_case:
		if entry.GetRemove() {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				f.Set(reflect.Zero(f.Type()))
				return nil
			}
		}

	case dpb.Entry_Test_case:
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() {
				return nil
			}
			val, err := decodeValue(entry.GetTest(), f.Type())
			if err != nil {
				return fmt.Errorf("decode test value: %w", err)
			}
			if f.Interface() != val.Interface() {
				return fmt.Errorf("test failed at field %q", name)
			}
			return nil
		}

	case dpb.Entry_Insert_case:
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() || !f.CanSet() {
				return nil
			}
			if f.Kind() == reflect.Pointer && !f.IsNil() {
				return nil
			}
			val, err := decodeValue(entry.GetInsert(), f.Type())
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			setField(f, val)
			return nil
		}

	case dpb.Entry_Assign_case:
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() || !f.CanSet() {
				return nil
			}
			val, err := decodeValue(entry.GetAssign(), f.Type())
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			setField(f, val)
			return nil
		}

	case dpb.Entry_Move_case:
		src_name := entry.GetMove().GetName()
		src := v.FieldByName(src_name)
		if !src.IsValid() {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				f.Set(reflect.Zero(f.Type()))
				return nil
			}
		} else {
			done := false
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				sv := src
				for sv.Kind() == reflect.Pointer {
					if sv.IsNil() {
						f.Set(reflect.Zero(f.Type()))
						return nil
					}
					sv = sv.Elem()
				}
				ft := f.Type()
				for ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				w, err := cast(sv, ft)
				if err != nil {
					return err
				}
				setField(f, w)
				if !done && src.CanSet() {
					src.Set(reflect.Zero(src.Type()))
					done = true
				}
				return nil
			}
		}

	case dpb.Entry_Copy_case:
		src_name := entry.GetCopy().GetName()
		src := v.FieldByName(src_name)
		if !src.IsValid() {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				f.Set(reflect.Zero(f.Type()))
				return nil
			}
		} else {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				sv := src
				for sv.Kind() == reflect.Pointer {
					if sv.IsNil() {
						f.Set(reflect.Zero(f.Type()))
						return nil
					}
					sv = sv.Elem()
				}
				ft := f.Type()
				for ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				w, err := cast(sv, ft)
				if err != nil {
					return err
				}
				setField(f, w)
				return nil
			}
		}

	case dpb.Entry_Nest_case:
		delta := entry.GetNest()
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() || !f.CanSet() {
				return nil
			}
			fv := f
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
			switch fv.Kind() {
			case reflect.Struct, reflect.Slice, reflect.Map:
				return o.PatchField(fv, delta)
			default:
				return fmt.Errorf("nested delta cannot be applied to %v", fv.Kind())
			}
		}

	default:
		return fmt.Errorf("unknown op: %q", entry.WhichKind())
	}

	errs := make([]error, 0, len(names))
	for _, name := range names {
		if err := op(name); err != nil {
			errs = append(errs, fmt.Errorf("[%s]: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
