package patchstruct

import (
	"fmt"
	"reflect"

	"github.com/lesomnus/protobuf-diff/dpb"
)

// decodeValue converts a *dpb.Value to a reflect.Value of the target type.
func decodeValue(v *dpb.Value, t reflect.Type) (reflect.Value, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	r := reflect.New(t).Elem()
	if v == nil || v.WhichKind() == dpb.Value_N_case {
		return r, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		switch v.WhichKind() {
		case dpb.Value_B_case:
			r.SetBool(v.GetB())
		case dpb.Value_I_case:
			r.SetBool(v.GetI() != 0)
		case dpb.Value_U_case:
			r.SetBool(v.GetU() != 0)
		default:
			return reflect.Value{}, fmt.Errorf("cannot convert %v to bool", v.WhichKind())
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v.WhichKind() {
		case dpb.Value_I_case:
			r.SetInt(v.GetI())
		case dpb.Value_U_case:
			r.SetInt(int64(v.GetU()))
		case dpb.Value_F_case:
			r.SetInt(int64(v.GetF()))
		case dpb.Value_B_case:
			if v.GetB() {
				r.SetInt(1)
			}
		default:
			return reflect.Value{}, fmt.Errorf("cannot convert %v to int", v.WhichKind())
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		switch v.WhichKind() {
		case dpb.Value_U_case:
			r.SetUint(v.GetU())
		case dpb.Value_I_case:
			r.SetUint(uint64(v.GetI()))
		case dpb.Value_F_case:
			r.SetUint(uint64(v.GetF()))
		default:
			return reflect.Value{}, fmt.Errorf("cannot convert %v to uint", v.WhichKind())
		}

	case reflect.Float32, reflect.Float64:
		switch v.WhichKind() {
		case dpb.Value_F_case:
			r.SetFloat(v.GetF())
		case dpb.Value_I_case:
			r.SetFloat(float64(v.GetI()))
		case dpb.Value_U_case:
			r.SetFloat(float64(v.GetU()))
		default:
			return reflect.Value{}, fmt.Errorf("cannot convert %v to float", v.WhichKind())
		}

	case reflect.String:
		if v.WhichKind() != dpb.Value_S_case {
			return reflect.Value{}, fmt.Errorf("cannot convert %v to string", v.WhichKind())
		}
		r.SetString(v.GetS())

	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			if v.WhichKind() != dpb.Value_X_case {
				return reflect.Value{}, fmt.Errorf("cannot convert %v to []byte", v.WhichKind())
			}
			cp := make([]byte, len(v.GetX()))
			copy(cp, v.GetX())
			r.Set(reflect.ValueOf(cp).Convert(t))
		} else {
			return reflect.Value{}, fmt.Errorf("unimplemented slice type: %v", t)
		}

	default:
		return reflect.Value{}, fmt.Errorf("unimplemented type: %v", t)
	}

	return r, nil
}

// setField sets dst to src, wrapping src in a pointer if dst is a pointer type.
// src must be the base (non-pointer) value.
func setField(dst, src reflect.Value) {
	if dst.Kind() == reflect.Pointer {
		ptr := reflect.New(dst.Type().Elem())
		ptr.Elem().Set(src)
		dst.Set(ptr)
	} else {
		dst.Set(src)
	}
}
