package patchstruct

import (
	"fmt"
	"reflect"
	"strconv"
)

type ErrInvalidCast struct {
	From, To reflect.Kind
}

func (e ErrInvalidCast) Error() string {
	return "invalid cast: " + e.From.String() + " -> " + e.To.String()
}

// cast converts v to type to. Uses reflect.Convert for numeric conversions.
func cast(v reflect.Value, to reflect.Type) (reflect.Value, error) {
	if v.Type() == to {
		return v, nil
	}
	if v.Type().ConvertibleTo(to) {
		return v.Convert(to), nil
	}

	from := v.Kind()
	tok := to.Kind()

	// string -> bytes
	if from == reflect.String && tok == reflect.Slice && to.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf([]byte(v.String())).Convert(to), nil
	}
	// bytes -> string
	if from == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 && tok == reflect.String {
		r := reflect.New(to).Elem()
		r.SetString(string(v.Bytes()))
		return r, nil
	}
	// string -> number
	if from == reflect.String {
		r := reflect.New(to).Elem()
		switch tok {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(v.String(), 10, 64)
			if err != nil {
				return reflect.Value{}, ErrInvalidCast{from, tok}
			}
			r.SetInt(i)
			return r, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u, err := strconv.ParseUint(v.String(), 10, 64)
			if err != nil {
				return reflect.Value{}, ErrInvalidCast{from, tok}
			}
			r.SetUint(u)
			return r, nil
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(v.String(), 64)
			if err != nil {
				return reflect.Value{}, ErrInvalidCast{from, tok}
			}
			r.SetFloat(f)
			return r, nil
		}
	}
	// number -> string
	if tok == reflect.String {
		r := reflect.New(to).Elem()
		switch from {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			r.SetString(strconv.FormatInt(v.Int(), 10))
			return r, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			r.SetString(strconv.FormatUint(v.Uint(), 10))
			return r, nil
		case reflect.Float32, reflect.Float64:
			r.SetString(fmt.Sprintf("%g", v.Float()))
			return r, nil
		}
	}

	return reflect.Value{}, ErrInvalidCast{from, tok}
}
