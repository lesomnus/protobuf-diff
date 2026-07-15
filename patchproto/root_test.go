package patchproto_test

import (
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

// mapStructVal builds a Value wrapping a Struct whose KeyValue keys are string
// map keys and values are string map values — the encoding used for a map root.
func mapStructVal(pairs ...[2]string) *dpb.Value {
	kvs := make([]*dpb.KeyValue, 0, len(pairs))
	for _, p := range pairs {
		kv := &dpb.KeyValue{}
		kv.SetKey(dpb.Field(p[0]))
		kv.SetValue(dpb.ValS(p[1]))
		kvs = append(kvs, kv)
	}
	s := &dpb.Struct{}
	s.SetFields(kvs)
	v := &dpb.Value{}
	v.SetM(s)
	return v
}

func TestRootReplaceMessage(t *testing.T) {
	t.Run("assign replaces the whole message", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		a.SetB_1(true) // present only in a; must be cleared
		a.SetRS_1([]string{"x", "y"})
		a.SetMSS(map[string]string{"k": "v"})

		b := &sample.Value{}
		b.SetS_2("bar")
		b.SetRI64_1([]int64{1, 2, 3})

		got, err := patchproto.Patched(a, dpb.NewDelta(dpb.ReplaceWith(b)))
		x.NoError(t, err)
		x.PbEq(t, b, got)
	})

	t.Run("round-trip with repeated and map message fields", func(t *testing.T) {
		inner1 := &sample.Value{}
		inner1.SetS_1("one")
		inner2 := &sample.Value{}
		inner2.SetEnum_1(sample.Level_LEVEL_HI)

		mapVal := &sample.Value{}
		mapVal.SetS_2("mapval")

		b := &sample.Value{}
		b.SetS_1("top")
		b.SetRM_1([]*sample.Value{inner1, inner2})
		b.SetMSM(map[string]*sample.Value{"a": mapVal})

		a := &sample.Value{}
		a.SetS_3("garbage") // present only in a; must be cleared

		got, err := patchproto.Patched(a, dpb.NewDelta(dpb.ReplaceWith(b)))
		x.NoError(t, err)
		x.PbEq(t, b, got)
	})

	t.Run("round-trip across scalar kinds", func(t *testing.T) {
		b := &sample.Value{}
		b.SetF64_1(3.14)
		b.SetF32_1(2.5)
		b.SetI64_1(-42)
		b.SetU64_1(42)
		b.SetI32_1(-7)
		b.SetU32_1(7)
		b.SetUx64_1(99)
		b.SetUx32_1(88)
		b.SetSi64_1(-1234)
		b.SetSx64_1(-5678)
		b.SetB_1(true)
		b.SetS_1("hello")
		b.SetBs_1([]byte{0x01, 0x02, 0x03})
		b.SetEnum_1(sample.Level_LEVEL_HI)

		a := &sample.Value{}
		a.SetS_2("garbage")

		got, err := patchproto.Patched(a, dpb.NewDelta(dpb.ReplaceWith(b)))
		x.NoError(t, err)
		x.PbEq(t, b, got)
	})

	t.Run("round-trip preserves edge-case string map keys", func(t *testing.T) {
		b := &sample.Value{}
		b.SetMSS(map[string]string{"": "empty", "5": "numeric", "k": "normal"})

		got, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(dpb.ReplaceWith(b)))
		x.NoError(t, err)
		x.PbEq(t, b, got)
	})

	t.Run("assign to a submessage via path", func(t *testing.T) {
		inner := &sample.Value{}
		inner.SetS_1("old")
		inner.SetS_2("old2")

		a := &sample.Value{}
		a.SetS_1("keep")
		a.SetM_1(inner)

		repl := &sample.Value{}
		repl.SetEnum_1(sample.Level_LEVEL_HI)

		e := dpb.ReplaceWith(repl)
		e.SetPath(dpb.PathOf(dpb.Field("m_1")))

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)

		wantInner := &sample.Value{}
		wantInner.SetEnum_1(sample.Level_LEVEL_HI)
		want := &sample.Value{}
		want.SetS_1("keep")
		want.SetM_1(wantInner)
		x.PbEq(t, want, got)
	})
}

func TestRootRemoveMessage(t *testing.T) {
	a := &sample.Value{}
	a.SetS_1("foo")
	a.SetRS_1([]string{"x", "y"})
	a.SetMSS(map[string]string{"k": "v"})

	e := &dpb.Entry{}
	e.SetRemove(true) // no targets → clear whole message

	got, err := patchproto.Patched(a, dpb.NewDelta(e))
	x.NoError(t, err)
	x.PbEq(t, &sample.Value{}, got)
}

func TestRootTestMessage(t *testing.T) {
	a := &sample.Value{}
	a.SetS_1("foo")
	a.SetRS_1([]string{"x", "y"})

	t.Run("pass", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetTest(dpb.ValM(a))
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)
	})
	t.Run("fail", func(t *testing.T) {
		other := &sample.Value{}
		other.SetS_1("different")
		e := &dpb.Entry{}
		e.SetTest(dpb.ValM(other))
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("empty value tests emptiness", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetTest(dpb.ValNull())
		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.NoError(t, err)
		_, err = patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})
}

func TestRootInsertMessage(t *testing.T) {
	b := &sample.Value{}
	b.SetS_1("inserted")

	t.Run("insert into empty message", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetInsert(dpb.ValM(b))
		got, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.NoError(t, err)
		x.PbEq(t, b, got)
	})
	t.Run("insert into non-empty message is no-op", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_2("existing")
		e := &dpb.Entry{}
		e.SetInsert(dpb.ValM(b))
		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)
		x.PbEq(t, a, got)
	})
}

func TestRootList(t *testing.T) {
	pathRS1 := func(e *dpb.Entry) { e.SetPath(dpb.PathOf(dpb.Field("r_s_1"))) }

	t.Run("assign replaces the whole list", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"a", "b"})

		e := &dpb.Entry{}
		pathRS1(e)
		e.SetAssign(dpb.ValL(dpb.ValS("x"), dpb.ValS("y"), dpb.ValS("z")))

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)

		want := &sample.Value{}
		want.SetRS_1([]string{"x", "y", "z"})
		x.PbEq(t, want, got)
	})

	t.Run("insert appends to the list", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"a", "b"})

		e := &dpb.Entry{}
		pathRS1(e)
		e.SetInsert(dpb.ValL(dpb.ValS("c")))

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)

		want := &sample.Value{}
		want.SetRS_1([]string{"a", "b", "c"})
		x.PbEq(t, want, got)
	})

	t.Run("remove clears the list", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"a", "b"})

		e := &dpb.Entry{}
		pathRS1(e)
		e.SetRemove(true)

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)
		x.PbEq(t, &sample.Value{}, got)
	})

	t.Run("test compares the whole list", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRS_1([]string{"a", "b"})

		pass := &dpb.Entry{}
		pathRS1(pass)
		pass.SetTest(dpb.ValL(dpb.ValS("a"), dpb.ValS("b")))
		_, err := patchproto.Patched(a, dpb.NewDelta(pass))
		x.NoError(t, err)

		fail := &dpb.Entry{}
		pathRS1(fail)
		fail.SetTest(dpb.ValL(dpb.ValS("a")))
		_, err = patchproto.Patched(a, dpb.NewDelta(fail))
		x.Error(t, err)
	})

	t.Run("null test checks emptiness", func(t *testing.T) {
		empty := &sample.Value{} // r_s_1 absent → empty list
		e := &dpb.Entry{}
		pathRS1(e)
		e.SetTest(dpb.ValNull())
		_, err := patchproto.Patched(empty, dpb.NewDelta(e))
		x.NoError(t, err)

		nonEmpty := &sample.Value{}
		nonEmpty.SetRS_1([]string{"a"})
		e2 := &dpb.Entry{}
		pathRS1(e2)
		e2.SetTest(dpb.ValNull())
		_, err = patchproto.Patched(nonEmpty, dpb.NewDelta(e2))
		x.Error(t, err)
	})
}

func TestRootKindMismatchReturnsError(t *testing.T) {
	t.Run("list element kind mismatch errors, not panic", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRM_1([]*sample.Value{{}}) // present message list

		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("r_m_1")))
		e.SetAssign(dpb.ValL(dpb.ValS("boom"))) // string into a message list

		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})

	t.Run("map value kind mismatch errors, not panic", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"x": "y"})

		kv := &dpb.KeyValue{}
		kv.SetKey(dpb.Field("z"))
		mv := &dpb.Value{}
		mv.SetM(&dpb.Struct{}) // message value into a string map
		kv.SetValue(mv)
		s := &dpb.Struct{}
		s.SetFields([]*dpb.KeyValue{kv})
		v := &dpb.Value{}
		v.SetM(s)

		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("m_s_s")))
		e.SetInsert(v)

		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})

	t.Run("message-root assign with scalar list field mismatch errors", func(t *testing.T) {
		// r_i64_1 is a repeated int64; feeding it a string element via a
		// full-message replace must error, not panic.
		kv := &dpb.KeyValue{}
		kv.SetKey(dpb.FieldNum(1003)) // r_i64_1
		kv.SetValue(dpb.ValL(dpb.ValS("nope")))
		s := &dpb.Struct{}
		s.SetFields([]*dpb.KeyValue{kv})
		mv := &dpb.Value{}
		mv.SetM(s)

		e := &dpb.Entry{}
		e.SetAssign(mv)

		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.Error(t, err)
	})
}

func TestRootMutateOnAbsentPathErrors(t *testing.T) {
	// A mutating root op reaching an unset container via a path must error, not
	// panic. (Reads via test are covered by the "null test checks emptiness"
	// cases, which navigate to absent containers successfully.)
	t.Run("assign to absent submessage", func(t *testing.T) {
		repl := &sample.Value{}
		repl.SetS_1("x")
		e := dpb.ReplaceWith(repl)
		e.SetPath(dpb.PathOf(dpb.Field("m_1"))) // m_1 unset
		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("assign to absent list", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("r_s_1"))) // r_s_1 unset
		e.SetAssign(dpb.ValL(dpb.ValS("a")))
		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("remove absent map", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("m_s_s"))) // m_s_s unset
		e.SetRemove(true)
		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("test through absent message-map key errors", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSM(map[string]*sample.Value{"present": {}})
		e := &dpb.Entry{}
		e.SetPath(dpb.PathOf(dpb.Field("m_s_m"), dpb.Field("missing"))) // absent key
		e.SetTest(dpb.ValS("x"))
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})
}

func TestFieldOpOnRepeatedTargetErrors(t *testing.T) {
	// A singular field-level op (assign/insert/test) targeting a repeated or map
	// field must error, not panic.
	t.Run("assign scalar to repeated field", func(t *testing.T) {
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1003))}) // r_i64_1
		e.SetAssign(dpb.ValI(5))
		_, err := patchproto.Patched(&sample.Value{}, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("test scalar against repeated field", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRI64_1([]int64{1})
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1003))}) // r_i64_1
		e.SetTest(dpb.ValI(1))
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})
	t.Run("remove on repeated field still clears", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRI64_1([]int64{1, 2})
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1003))}) // r_i64_1
		e.SetRemove(true)
		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)
		x.PbEq(t, &sample.Value{}, got)
	})

	t.Run("move with repeated source errors", func(t *testing.T) {
		a := &sample.Value{}
		a.SetRI64_1([]int64{1, 2, 3})
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("s_1")})
		e.SetMove(dpb.Field("r_i64_1")) // repeated source
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})

	t.Run("copy with map source errors", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSM(map[string]*sample.Value{"k": {}})
		e := &dpb.Entry{}
		e.SetTargets([]*dpb.Segment{dpb.SegName("m_1")})
		e.SetCopy(dpb.Field("m_s_m")) // map source
		_, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.Error(t, err)
	})
}

func TestRootMap(t *testing.T) {
	pathMSS := func(e *dpb.Entry) { e.SetPath(dpb.PathOf(dpb.Field("m_s_s"))) }

	t.Run("assign replaces the whole map", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"a": "1", "b": "2"})

		e := &dpb.Entry{}
		pathMSS(e)
		e.SetAssign(mapStructVal([2]string{"x", "9"}))

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)

		want := &sample.Value{}
		want.SetMSS(map[string]string{"x": "9"})
		x.PbEq(t, want, got)
	})

	t.Run("insert merges only absent keys", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"a": "1", "b": "2"})

		e := &dpb.Entry{}
		pathMSS(e)
		e.SetInsert(mapStructVal([2]string{"b", "X"}, [2]string{"c", "3"}))

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)

		want := &sample.Value{}
		want.SetMSS(map[string]string{"a": "1", "b": "2", "c": "3"}) // b unchanged
		x.PbEq(t, want, got)
	})

	t.Run("remove clears the map", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"a": "1", "b": "2"})

		e := &dpb.Entry{}
		pathMSS(e)
		e.SetRemove(true)

		got, err := patchproto.Patched(a, dpb.NewDelta(e))
		x.NoError(t, err)
		x.PbEq(t, &sample.Value{}, got)
	})

	t.Run("test compares the whole map", func(t *testing.T) {
		a := &sample.Value{}
		a.SetMSS(map[string]string{"a": "1", "b": "2"})

		pass := &dpb.Entry{}
		pathMSS(pass)
		pass.SetTest(mapStructVal([2]string{"a", "1"}, [2]string{"b", "2"}))
		_, err := patchproto.Patched(a, dpb.NewDelta(pass))
		x.NoError(t, err)

		fail := &dpb.Entry{}
		pathMSS(fail)
		fail.SetTest(mapStructVal([2]string{"a", "1"}))
		_, err = patchproto.Patched(a, dpb.NewDelta(fail))
		x.Error(t, err)
	})

	t.Run("null test checks emptiness", func(t *testing.T) {
		empty := &sample.Value{} // m_s_s absent → empty map
		e := &dpb.Entry{}
		pathMSS(e)
		e.SetTest(dpb.ValNull())
		_, err := patchproto.Patched(empty, dpb.NewDelta(e))
		x.NoError(t, err)

		nonEmpty := &sample.Value{}
		nonEmpty.SetMSS(map[string]string{"a": "1"})
		e2 := &dpb.Entry{}
		pathMSS(e2)
		e2.SetTest(dpb.ValNull())
		_, err = patchproto.Patched(nonEmpty, dpb.NewDelta(e2))
		x.Error(t, err)
	})
}
