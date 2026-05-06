package ref_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/ref"
)

func TestRefEncoding(t *testing.T) {
	vs := []uint64{
		0x00,
		0x01,
		0x7f,
		0x80,
		0xff,
		0x100,
		0xffff,
		0x10000,
		0xffffff,
		0x1000000,
	}
	for _, v := range vs {
		b := ref.EncodeInt(v)
		w := ref.DecodeInt(b)
		x.Eq(t, v, w)
	}
}
