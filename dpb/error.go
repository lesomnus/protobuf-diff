package dpb

import "google.golang.org/protobuf/reflect/protoreflect"

type ErrInvalidCast struct {
	From, To protoreflect.Kind
}

func (e ErrInvalidCast) Error() string {
	return "invalid cast: " + e.From.String() + " -> " + e.To.String()
}
