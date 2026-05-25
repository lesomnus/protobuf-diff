package dpb

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lesomnus/protobuf-diff/jsonpatch"
)

// FromJsonPatch converts a JSON Patch document (RFC 6902) into a Delta.
//
// Path segments that are valid integers are treated as list indices.
// The special "-" token is treated as -1 (append).
// String segments are treated as field names or map keys.
//
// add on a list index → insert; add on a string key → assign (create-or-replace).
// replace always maps to assign (creates if absent).
// copy and move are only supported within the same container (identical parent paths).
// test ops are silently ignored.
func FromJsonPatch(doc jsonpatch.Doc) (*Delta, error) {
	var entries []*Entry
	for i, op := range doc {
		es, err := fromJsonPatchOp(op)
		if err != nil {
			return nil, fmt.Errorf("doc[%d] %q: %w", i, op.Op, err)
		}
		entries = append(entries, es...)
	}
	return NewDelta(entries...), nil
}

func fromJsonPatchOp(op jsonpatch.Op) ([]*Entry, error) {
	switch op.Op {
	case "add":
		return fromJsonPatchAdd(op)
	case "remove":
		return fromJsonPatchRemove(op)
	case "replace":
		return fromJsonPatchReplace(op)
	case "move":
		return fromJsonPatchMove(op)
	case "copy":
		return fromJsonPatchCopy(op)
	case "test":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown op: %q", op.Op)
	}
}

func fromJsonPatchAdd(op jsonpatch.Op) ([]*Entry, error) {
	segs := parseJsonPointer(op.Path)
	if len(segs) == 0 {
		return nil, fmt.Errorf("path must not be root")
	}
	val, err := jsonToValue(op.Value)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}

	parent := segs[:len(segs)-1]
	last := segs[len(segs)-1]

	e := &Entry{}
	e.SetPath(buildJsonPath(parent))
	e.SetTargets([]*Segment{jsonSegFromAny(last)})
	if _, ok := last.(int); ok {
		e.SetInsert(val) // list: insert-before
	} else {
		e.SetAssign(val) // message/map: create-or-replace
	}
	return []*Entry{e}, nil
}

func fromJsonPatchRemove(op jsonpatch.Op) ([]*Entry, error) {
	segs := parseJsonPointer(op.Path)
	if len(segs) == 0 {
		return nil, fmt.Errorf("path must not be root")
	}

	parent := segs[:len(segs)-1]
	last := segs[len(segs)-1]

	e := &Entry{}
	e.SetPath(buildJsonPath(parent))
	e.SetTargets([]*Segment{jsonSegFromAny(last)})
	e.SetRemove(true)
	return []*Entry{e}, nil
}

func fromJsonPatchReplace(op jsonpatch.Op) ([]*Entry, error) {
	segs := parseJsonPointer(op.Path)
	if len(segs) == 0 {
		return nil, fmt.Errorf("path must not be root")
	}
	val, err := jsonToValue(op.Value)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}

	parent := segs[:len(segs)-1]
	last := segs[len(segs)-1]

	e := &Entry{}
	e.SetPath(buildJsonPath(parent))
	e.SetTargets([]*Segment{jsonSegFromAny(last)})
	e.SetAssign(val)
	return []*Entry{e}, nil
}

func fromJsonPatchMove(op jsonpatch.Op) ([]*Entry, error) {
	fromSegs := parseJsonPointer(op.From)
	pathSegs := parseJsonPointer(op.Path)
	parent, fromSeg, pathSeg, err := sameContainerSegs(fromSegs, pathSegs)
	if err != nil {
		return nil, err
	}

	// JSON Patch move: path index is in the post-removal array.
	// Delta move operates on the original array, so adjust when moving forward.
	if fromIdx, ok := fromSeg.(int); ok {
		if pathIdx, ok := pathSeg.(int); ok {
			if fromIdx >= 0 && pathIdx >= 0 && fromIdx < pathIdx {
				pathSeg = pathIdx + 1
			}
		}
	}

	e := &Entry{}
	e.SetPath(buildJsonPath(parent))
	e.SetTargets([]*Segment{jsonSegFromAny(pathSeg)})
	e.SetMove(jsonFieldSegFromAny(fromSeg))
	return []*Entry{e}, nil
}

func fromJsonPatchCopy(op jsonpatch.Op) ([]*Entry, error) {
	fromSegs := parseJsonPointer(op.From)
	pathSegs := parseJsonPointer(op.Path)
	parent, fromSeg, pathSeg, err := sameContainerSegs(fromSegs, pathSegs)
	if err != nil {
		return nil, err
	}

	e := &Entry{}
	e.SetPath(buildJsonPath(parent))
	e.SetTargets([]*Segment{jsonSegFromAny(pathSeg)})
	e.SetCopy(jsonFieldSegFromAny(fromSeg))
	return []*Entry{e}, nil
}

func sameContainerSegs(fromSegs, pathSegs []any) ([]any, any, any, error) {
	if len(fromSegs) == 0 || len(pathSegs) == 0 {
		return nil, nil, nil, fmt.Errorf("path must not be root")
	}
	fromParent := fromSegs[:len(fromSegs)-1]
	pathParent := pathSegs[:len(pathSegs)-1]
	if len(fromParent) != len(pathParent) {
		return nil, nil, nil, fmt.Errorf("cross-container operations are not supported")
	}
	for i := range fromParent {
		if fromParent[i] != pathParent[i] {
			return nil, nil, nil, fmt.Errorf("cross-container operations are not supported")
		}
	}
	return fromParent, fromSegs[len(fromSegs)-1], pathSegs[len(pathSegs)-1], nil
}

func buildJsonPath(segs []any) *Path {
	fsegs := make([]*FieldSegment, len(segs))
	for i, seg := range segs {
		fsegs[i] = jsonFieldSegFromAny(seg)
	}
	return PathOf(fsegs...)
}

func jsonSegFromAny(seg any) *Segment {
	switch s := seg.(type) {
	case int:
		return SegIndex(int64(s))
	case string:
		return SegName(s)
	}
	return nil
}

func jsonFieldSegFromAny(seg any) *FieldSegment {
	switch s := seg.(type) {
	case int:
		return FieldNum(int64(s))
	case string:
		return Field(s)
	}
	return nil
}

// parseJsonPointer splits a JSON Pointer into typed segments.
// Numeric strings are returned as int; "-" is returned as int(-1); others as string.
func parseJsonPointer(path string) []any {
	var segs []any
	for seg := range jsonpatch.Location(path).Seq() {
		if seg == "-" {
			segs = append(segs, -1)
		} else if n, err := strconv.Atoi(seg); err == nil {
			segs = append(segs, n)
		} else {
			segs = append(segs, seg)
		}
	}
	return segs
}

func jsonToValue(v json.RawMessage) (*Value, error) {
	if len(v) == 0 {
		return ValNull(), nil
	}
	switch v[0] {
	case '"':
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, err
		}
		return ValS(s), nil
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return nil, err
		}
		return ValB(b), nil
	case 'n':
		return ValNull(), nil
	default:
		var i int64
		if err := json.Unmarshal(v, &i); err == nil {
			return ValI(i), nil
		}
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			return ValF(f), nil
		}
		return nil, fmt.Errorf("unsupported JSON value: %s", v)
	}
}
