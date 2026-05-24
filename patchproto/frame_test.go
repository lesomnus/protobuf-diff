package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
	"github.com/lesomnus/protobuf-diff/target"
	"google.golang.org/protobuf/reflect/protoreflect"
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
		d.AppendTargets(target.Fields(109)) // s_1
		d.SetAssigned(dpb.String("new"))

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
		d.AppendTargets(target.Fields(111)) // m_1
		d.SetAssigned(dpb.Message(msg_y))

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
		d.AppendTargets(target.Fields(109, 209)) // s_1, s_2
		d.SetAssigned(dpb.String("new"))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 2, len(*captures))
		x.Eq(t, "old1", (*captures)[0].before.String())
		x.Eq(t, "new", (*captures)[0].after.String())
		x.Eq(t, "old2", (*captures)[1].before.String())
		x.Eq(t, "new", (*captures)[1].after.String())
	})
	t.Run("multiple message fields separate entries", func(t *testing.T) {
		m1_x := &sample.Value{}
		m1_x.SetS_1("m1_old")
		m1_y := &sample.Value{}
		m1_y.SetS_1("m1_new")

		m2_x := &sample.Value{}
		m2_x.SetS_1("m2_old")
		m2_y := &sample.Value{}
		m2_y.SetS_1("m2_new")

		a := &sample.Value{}
		a.SetM_1(m1_x)
		a.SetM_2(m2_x)

		d1 := &dpb.Entry{}
		d1.AppendTargets(target.Fields(111)) // m_1
		d1.SetAssigned(dpb.Message(m1_y))

		d2 := &dpb.Entry{}
		d2.AppendTargets(target.Fields(211)) // m_2
		d2.SetAssigned(dpb.Message(m2_y))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d1, d2), hook)
		x.NoError(t, err)

		x.Eq(t, 2, len(*captures))
		x.Eq(t, `s_1:"m1_old"`, (*captures)[0].before.String())
		x.Eq(t, `s_1:"m1_new"`, (*captures)[0].after.String())
		x.Eq(t, `s_1:"m2_old"`, (*captures)[1].before.String())
		x.Eq(t, `s_1:"m2_new"`, (*captures)[1].after.String())
	})
	t.Run("message field not set before", func(t *testing.T) {
		a := &sample.Value{}
		// m_1 is not set — message fields with presence return invalid value from c.Get

		d := &dpb.Entry{}
		d.AppendTargets(target.Fields(111)) // m_1
		d.SetAssigned(dpb.Message(&sample.Value{}))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(d), hook)
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String()) // not set → invalid value
		x.Eq(t, "", (*captures)[0].after.String())       // empty message → empty text
	})
}

// TestFrameInsert verifies Frame.String() when elements are added to lists and maps.
// For inserts, before is always "<nil>" because no value existed at that position.
func TestFrameInsert(t *testing.T) {
	// wrap an inner entry targeting the given list/map field
	wrapField := func(fieldNum protoreflect.FieldNumber, inner *dpb.Entry) *dpb.Entry {
		outer := &dpb.Entry{}
		outer.AppendTargets(target.Fields(fieldNum))
		outer.SetNested(dpb.NewDelta(inner))
		return outer
	}

	t.Run("list scalar insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"foo", "bar"})

		inner := &dpb.Entry{}
		inner.SetNoUpdate(true)
		inner.AppendTargets(target.Indices(1)) // insert before index 1
		inner.SetAssigned(dpb.String("baz"))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapField(1009, inner)), hook) // r_s_1
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String()) // no element existed at this position
		x.Eq(t, "baz", (*captures)[0].after.String())
	})
	t.Run("list message insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRM_1([]*sample.Value{})

		new_elem := &sample.Value{}
		new_elem.SetS_1("inserted")

		inner := &dpb.Entry{}
		inner.SetNoUpdate(true)
		inner.AppendTargets(target.Indices(0))
		inner.SetAssigned(dpb.Message(new_elem))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapField(1011, inner)), hook) // r_m_1
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String())
		x.Eq(t, `s_1:"inserted"`, (*captures)[0].after.String())
	})
	t.Run("map scalar insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{}) // empty map

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("hello"))
		inner.SetAssigned(dpb.String("world"))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapField(10909, inner)), hook) // m_s_s
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String()) // key didn't exist before
		x.Eq(t, "world", (*captures)[0].after.String())
	})
	t.Run("map message insert", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSM(map[string]*sample.Value{})

		new_val := &sample.Value{}
		new_val.SetS_1("inserted")

		inner := &dpb.Entry{}
		inner.AppendTargets(target.StringKeys("key"))
		inner.SetAssigned(dpb.Message(new_val))

		hook, captures := captureFrames()
		_, err := patchproto.Patched(a, dpb.NewDelta(wrapField(10911, inner)), hook) // m_s_m
		x.NoError(t, err)

		x.Eq(t, 1, len(*captures))
		x.Eq(t, "<nil>", (*captures)[0].before.String())
		x.Eq(t, `s_1:"inserted"`, (*captures)[0].after.String())
	})
}
