package patchstruct

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/ref"
	"google.golang.org/protobuf/encoding/protowire"
)

func decodeFieldNames(data []byte) ([]string, error) {
	var names []string
	for len(data) > 0 {
		s, n := protowire.ConsumeString(data)
		if n < 0 {
			return nil, fmt.Errorf("invalid string encoding: %w", protowire.ParseError(n))
		}
		names = append(names, s)
		data = data[n:]
	}
	return names, nil
}

func (o PatchOption) patchStruct(v reflect.Value, entry *dpb.Entry) error {
	names, err := decodeFieldNames(entry.GetTargets())
	if err != nil {
		return fmt.Errorf("decode targets: %w", err)
	}
	if len(names) == 0 {
		return nil
	}

	check := func(f reflect.Value) bool {
		exists := f.Kind() != reflect.Pointer || !f.IsNil()
		if !exists && entry.GetNoInsert() {
			return false
		}
		if exists && entry.GetNoUpdate() {
			return false
		}
		return true
	}

	op := func(name string) error { return nil }

	switch entry.WhichKind() {
	case dpb.Entry_Deleted_case:
		if entry.GetDeleted() {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				f.Set(reflect.Zero(f.Type()))
				return nil
			}
		}

	case dpb.Entry_Assigned_case:
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() || !f.CanSet() {
				return nil
			}
			if !check(f) {
				return nil
			}
			val, err := decodeValue(entry.GetAssigned(), f.Type())
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			setField(f, val)
			return nil
		}

	case dpb.Entry_Merged_case:
		return fmt.Errorf("unimplemented: %q", entry.WhichKind())

	case dpb.Entry_Copied_case:
		src_name := ref.DecodeString(entry.GetCopied())
		src := v.FieldByName(src_name)
		if !src.IsValid() {
			// Source not found: clear targets.
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				if !check(f) {
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
				if !check(f) {
					return nil
				}
				// Dereference pointer source.
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

	case dpb.Entry_Scattered_case:
		src_name := ref.DecodeString(entry.GetScattered())
		src := v.FieldByName(src_name)
		if !src.IsValid() {
			op = func(name string) error {
				f := v.FieldByName(name)
				if !f.IsValid() || !f.CanSet() {
					return nil
				}
				if !check(f) {
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
				if !check(f) {
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

	case dpb.Entry_Swapped_case:
		dst_name := ref.DecodeString(entry.GetSwapped())

		op = func(name string) error {
			fa := v.FieldByName(name)
			fb := v.FieldByName(dst_name)
			if !fa.IsValid() || !fa.CanSet() || !fb.IsValid() || !fb.CanSet() {
				return nil
			}

			tat := fa.Type()
			for tat.Kind() == reflect.Pointer {
				tat = tat.Elem()
			}

			tbt := fb.Type()
			for tbt.Kind() == reflect.Pointer {
				tbt = tbt.Elem()
			}

			va := fa
			for va.Kind() == reflect.Pointer {
				if va.IsNil() {
					break
				}
				va = va.Elem()
			}

			vb := fb
			for vb.Kind() == reflect.Pointer {
				if vb.IsNil() {
					break
				}
				vb = vb.Elem()
			}

			// Snapshot values before modifying to avoid aliasing via reflect.Value.
			snap_a := reflect.New(tat).Elem()
			snap_a.Set(va)

			snap_b := reflect.New(tbt).Elem()
			snap_b.Set(vb)

			wa, err := cast(snap_a, tbt)
			if err != nil {
				return fmt.Errorf("cast src: %w", err)
			}

			wb, err := cast(snap_b, tat)
			if err != nil {
				return fmt.Errorf("cast dst: %w", err)
			}

			setField(fa, wb)
			setField(fb, wa)
			return nil
		}

	case dpb.Entry_Nested_case:
		delta := entry.GetNested()
		op = func(name string) error {
			f := v.FieldByName(name)
			if !f.IsValid() || !f.CanSet() {
				return nil
			}
			if !check(f) {
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
