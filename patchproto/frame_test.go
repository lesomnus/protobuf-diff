package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

type frameCapture struct {
	path   []dpb.PathEntry
	before patchproto.Frame
	after  patchproto.Frame
}

func captureFrames() (patchproto.Option, *[]frameCapture) {
	var captures []frameCapture
	opt := patchproto.WithHook(func(p []dpb.PathEntry, b, a dpb.Frame, _ *dpb.Entry) {
		cp := make([]dpb.PathEntry, len(p))
		copy(cp, p)
		captures = append(captures, frameCapture{
			path:   cp,
			before: b.(patchproto.Frame),
			after:  a.(patchproto.Frame),
		})
	})
	return opt, &captures
}

func TestFrameString(t *testing.T) {
	t.Run("single scalar field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("old")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(109))}) // s_1
		d.SetAssign(dpb.ValS("new"))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		c := (*captures)[0]
		x.Eq(t, "old", c.before.String())
		x.Eq(t, "new", c.after.String())
	})
	t.Run("single message field", func(t *testing.T) {
		msg_x := &sample.Value{}
		msg_x.SetS_1("before")

		msg_y := &sample.Value{}
		msg_y.SetS_1("after")

		a := &sample.Value{}
		a.SetM_1(msg_x)

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(111))}) // m_1
		d.SetAssign(dpb.ValM(msg_y))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		c := (*captures)[0]
		x.Eq(t, `s_1:"before"`, c.before.String())
		x.Eq(t, `s_1:"after"`, c.after.String())
	})
	t.Run("multiple scalar fields same entry", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("old1")
		a.SetS_2("old2")

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{
			dpb.SegField(dpb.FieldNum(109)), // s_1
			dpb.SegField(dpb.FieldNum(209)), // s_2
		})
		d.SetAssign(dpb.ValS("new"))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 2, len(*captures))
		x.Eq(t, "old1", (*captures)[0].before.String())
		x.Eq(t, "new", (*captures)[0].after.String())
		x.Eq(t, "old2", (*captures)[1].before.String())
		x.Eq(t, "new", (*captures)[1].after.String())
	})
	t.Run("message field not set before", func(t *testing.T) {
		a := &sample.Value{}
		// m_1 is not set — message fields with presence return invalid value from c.Get

		d := &dpb.Entry{}
		d.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(111))}) // m_1
		d.SetAssign(dpb.ValM(&sample.Value{}))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String()) // not set → invalid value
		x.Eq(t, "", (*captures)[0].after.String())       // empty message → empty text
	})
}

// TestFrameInsert verifies Frame.String() when elements are added to lists and maps.
func TestFrameInsert(t *testing.T) {
	t.Run("list scalar insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar"})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegIndex(1)}) // insert before index 1
		inner.SetInsert(dpb.ValS("baz"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1009))}) // r_s_1
		outer.SetNest(dpb.NewDelta(inner))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String())
		x.Eq(t, "baz", (*captures)[0].after.String())
	})
	t.Run("map scalar insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{})

		inner := &dpb.Entry{}
		inner.SetTargets([]*dpb.Segment{dpb.SegName("hello")})
		inner.SetAssign(dpb.ValS("world"))

		outer := &dpb.Entry{}
		outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(10909))}) // m_s_s
		outer.SetNest(dpb.NewDelta(inner))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(outer), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String())
		x.Eq(t, "world", (*captures)[0].after.String())
	})
}
