package dpb_test

import (
	"encoding/json"
	"testing"

	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/internal/sample"
	"github.com/lesomnus/protobuf-diff/internal/x"
	"github.com/lesomnus/protobuf-diff/jsonpatch"
	"github.com/lesomnus/protobuf-diff/patchproto"
	"google.golang.org/protobuf/proto"
)

func TestFromJsonPatch(t *testing.T) {
	apply := func(t *testing.T, msg proto.Message, doc jsonpatch.Doc) proto.Message {
		t.Helper()

		delta, err := dpb.FromJsonPatch(doc)
		x.NoError(t, err)

		got, err := patchproto.Patched(msg, delta)
		x.NoError(t, err)

		return got
	}
	jv := func(v any) json.RawMessage {
		t.Helper()
		b, err := json.Marshal(v)
		x.NoError(t, err)
		return json.RawMessage(b)
	}

	t.Run("add", func(t *testing.T) {
		t.Run("string field", func(t *testing.T) {
			a := &sample.Value{}
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/s_1", Value: jv("foo")},
			})
			want := &sample.Value{}
			want.SetS_1("foo")
			x.PbEq(t, want, got)
		})
		t.Run("existing field is overwritten", func(t *testing.T) {
			a := &sample.Value{}
			a.SetS_1("foo")
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/s_1", Value: jv("bar")},
			})
			want := &sample.Value{}
			want.SetS_1("bar")
			x.PbEq(t, want, got)
		})
		t.Run("bool field", func(t *testing.T) {
			a := &sample.Value{}
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/b_1", Value: jv(true)},
			})
			want := &sample.Value{}
			want.SetB_1(true)
			x.PbEq(t, want, got)
		})
		t.Run("nested field", func(t *testing.T) {
			inner := &sample.Value{}
			a := &sample.Value{}
			a.SetM_1(inner)
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/m_1/s_2", Value: jv("bar")},
			})

			want_inner := &sample.Value{}
			want_inner.SetS_2("bar")
			want := &sample.Value{}
			want.SetM_1(want_inner)

			x.PbEq(t, want, got)
		})
		t.Run("list append with -", func(t *testing.T) {
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/r_s_1/-", Value: jv("c")},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"a", "b", "c"})
			x.PbEq(t, want, got)
		})
		t.Run("list insert at index", func(t *testing.T) {
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "add", Path: "/r_s_1/1", Value: jv("x")},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"a", "x", "b", "c"})
			x.PbEq(t, want, got)
		})
	})
	t.Run("remove", func(t *testing.T) {
		t.Run("message field", func(t *testing.T) {
			a := &sample.Value{}
			a.SetS_1("foo")
			a.SetS_2("bar")
			got := apply(t, a, jsonpatch.Doc{
				{Op: "remove", Path: "/s_1"},
			})
			want := &sample.Value{}
			want.SetS_2("bar")
			x.PbEq(t, want, got)
		})
		t.Run("list element", func(t *testing.T) {
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "remove", Path: "/r_s_1/1"},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"a", "c"})
			x.PbEq(t, want, got)
		})
		t.Run("map entry", func(t *testing.T) {
			a := &sample.Value{}
			a.SetMSS(map[string]string{"a": "1", "b": "2"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "remove", Path: "/m_s_s/a"},
			})
			want := &sample.Value{}
			want.SetMSS(map[string]string{"b": "2"})
			x.PbEq(t, want, got)
		})
	})
	t.Run("replace", func(t *testing.T) {
		t.Run("existing field", func(t *testing.T) {
			a := &sample.Value{}
			a.SetS_1("foo")
			got := apply(t, a, jsonpatch.Doc{
				{Op: "replace", Path: "/s_1", Value: jv("bar")},
			})
			want := &sample.Value{}
			want.SetS_1("bar")
			x.PbEq(t, want, got)
		})
		t.Run("absent optional field is assigned", func(t *testing.T) {
			a := &sample.Value{}
			// replace maps to assign which always sets, even if the field was absent
			got := apply(t, a, jsonpatch.Doc{
				{Op: "replace", Path: "/opt_s", Value: jv("bar")},
			})
			want := &sample.Value{}
			want.SetOptS("bar")
			x.PbEq(t, want, got)
		})
		t.Run("list element", func(t *testing.T) {
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "replace", Path: "/r_s_1/1", Value: jv("x")},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"a", "x", "c"})
			x.PbEq(t, want, got)
		})
		t.Run("map value", func(t *testing.T) {
			a := &sample.Value{}
			a.SetMSS(map[string]string{"a": "1", "b": "2"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "replace", Path: "/m_s_s/a", Value: jv("99")},
			})
			want := &sample.Value{}
			want.SetMSS(map[string]string{"a": "99", "b": "2"})
			x.PbEq(t, want, got)
		})
	})
	t.Run("copy", func(t *testing.T) {
		t.Run("list element to another index", func(t *testing.T) {
			// copy /r_s_1/0 to /r_s_1/2 inserts a copy at index 2; source stays
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "copy", From: "/r_s_1/0", Path: "/r_s_1/2"},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"a", "b", "a", "c"})
			x.PbEq(t, want, got)
		})
		t.Run("list element to earlier index", func(t *testing.T) {
			// copy /r_s_1/2 to /r_s_1/0 inserts a copy at index 0; source stays
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "copy", From: "/r_s_1/2", Path: "/r_s_1/0"},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"c", "a", "b", "c"})
			x.PbEq(t, want, got)
		})
		t.Run("map entry", func(t *testing.T) {
			a := &sample.Value{}
			a.SetMSS(map[string]string{"a": "hello", "b": "world"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "copy", From: "/m_s_s/a", Path: "/m_s_s/c"},
			})
			want := &sample.Value{}
			want.SetMSS(map[string]string{"a": "hello", "b": "world", "c": "hello"})
			x.PbEq(t, want, got)
		})
	})
	t.Run("move", func(t *testing.T) {
		t.Run("list element forward", func(t *testing.T) {
			// move /r_s_1/0 to /r_s_1/2: remove a, insert at post-remove index 2
			// [a,b,c,d] -> remove a -> [b,c,d] -> insert a at 2 -> [b,c,a,d]
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c", "d"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "move", From: "/r_s_1/0", Path: "/r_s_1/2"},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"b", "c", "a", "d"})
			x.PbEq(t, want, got)
		})
		t.Run("list element backward", func(t *testing.T) {
			// move /r_s_1/2 to /r_s_1/0: remove c, insert at post-remove index 0
			// [a,b,c,d] -> remove c -> [a,b,d] -> insert c at 0 -> [c,a,b,d]
			a := &sample.Value{}
			a.SetRS_1([]string{"a", "b", "c", "d"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "move", From: "/r_s_1/2", Path: "/r_s_1/0"},
			})
			want := &sample.Value{}
			want.SetRS_1([]string{"c", "a", "b", "d"})
			x.PbEq(t, want, got)
		})
		t.Run("map entry rename", func(t *testing.T) {
			a := &sample.Value{}
			a.SetMSS(map[string]string{"a": "hello", "b": "world"})
			got := apply(t, a, jsonpatch.Doc{
				{Op: "move", From: "/m_s_s/a", Path: "/m_s_s/c"},
			})
			want := &sample.Value{}
			want.SetMSS(map[string]string{"b": "world", "c": "hello"})
			x.PbEq(t, want, got)
		})
	})
	t.Run("test op is ignored", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		got := apply(t, a, jsonpatch.Doc{
			{Op: "test", Path: "/s_1", Value: jv("foo")},
		})
		x.PbEq(t, a, got)
	})
	t.Run("multiple ops", func(t *testing.T) {
		a := &sample.Value{}
		a.SetS_1("foo")
		a.SetS_2("bar")
		got := apply(t, a, jsonpatch.Doc{
			{Op: "replace", Path: "/s_1", Value: jv("FOO")},
			{Op: "remove", Path: "/s_2"},
			{Op: "add", Path: "/s_3", Value: jv("baz")},
		})
		want := &sample.Value{}
		want.SetS_1("FOO")
		want.SetS_3("baz")
		x.PbEq(t, want, got)
	})
	t.Run("cross-container move returns error", func(t *testing.T) {
		doc := jsonpatch.Doc{
			{Op: "move", From: "/s_1", Path: "/m_1/s_2"},
		}
		_, err := dpb.FromJsonPatch(doc)
		x.Error(t, err)
	})
	t.Run("cross-container copy returns error", func(t *testing.T) {
		doc := jsonpatch.Doc{
			{Op: "copy", From: "/s_1", Path: "/m_1/s_2"},
		}
		_, err := dpb.FromJsonPatch(doc)
		x.Error(t, err)
	})
}
