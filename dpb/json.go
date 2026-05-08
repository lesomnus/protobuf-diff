package dpb

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lesomnus/protobuf-diff/jsonpatch"
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

// FromJsonPatch converts a JSON Patch document (RFC 6902) into a Delta.
//
// Path segments that are valid integers are encoded as signed integers in the Delta path so patchproto can use them as list indices.
// The special "-" token is encoded as -1 (append).
// String segments are kept as strings.
//
// Value encoding for add/replace: strings are stored as raw bytes, booleans as varint, integers as varint, floats as fixed64.
// Objects and arrays are not supported and will return an error.
//
// copy and move ops are only supported within the same container (identical parent paths).
// Cross-container operations return an error.
//
// test ops are silently ignored since they have no mutating equivalent.
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
	value, err := jsonToBytes(op.Value)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}

	e := &Entry{}
	e.SetPath(buildDeltaPath(segs).Value())
	e.SetAssigned(value)
	if len(segs) > 0 {
		if _, ok := segs[len(segs)-1].(int); ok {
			e.SetNoUpdate(true)
		}
	}
	return []*Entry{e}, nil
}

func fromJsonPatchRemove(op jsonpatch.Op) ([]*Entry, error) {
	segs := parseJsonPointer(op.Path)
	e := &Entry{}
	e.SetPath(buildDeltaPath(segs).Value())
	e.SetDeleted(true)
	return []*Entry{e}, nil
}

func fromJsonPatchReplace(op jsonpatch.Op) ([]*Entry, error) {
	segs := parseJsonPointer(op.Path)
	value, err := jsonToBytes(op.Value)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}

	e := &Entry{}
	e.SetPath(buildDeltaPath(segs).Value())
	e.SetAssigned(value)
	e.SetNoInsert(true)
	return []*Entry{e}, nil
}

func fromJsonPatchMove(op jsonpatch.Op) ([]*Entry, error) {
	fromSegs := parseJsonPointer(op.From)
	pathSegs := parseJsonPointer(op.Path)
	parentPath, fromSeg, pathSeg, err := samePContainerSegs(fromSegs, pathSegs)
	if err != nil {
		return nil, err
	}

	e := &Entry{}
	e.SetPath(parentPath.Value())
	// For list context: JSON Patch move path index is in the post-removal array,
	// but Delta Scattered operates on the original array. Adjust when from < path.
	if fromIdx, ok := fromSeg.(int); ok {
		if pathIdx, ok := pathSeg.(int); ok {
			e.SetNoUpdate(true)
			if fromIdx >= 0 && pathIdx >= 0 && fromIdx < pathIdx {
				pathSeg = pathIdx + 1
			}
		}
	}
	appendJsonTarget(e, pathSeg)
	e.ScatteredFrom(makeJsonRef(fromSeg))
	return []*Entry{e}, nil
}

func fromJsonPatchCopy(op jsonpatch.Op) ([]*Entry, error) {
	fromSegs := parseJsonPointer(op.From)
	pathSegs := parseJsonPointer(op.Path)
	parentPath, fromSeg, pathSeg, err := samePContainerSegs(fromSegs, pathSegs)
	if err != nil {
		return nil, err
	}

	e := &Entry{}
	e.SetPath(parentPath.Value())
	// For list context: set insert mode so the copied value is inserted, not replaced.
	if _, ok := fromSeg.(int); ok {
		if _, ok := pathSeg.(int); ok {
			e.SetNoUpdate(true)
		}
	}
	appendJsonTarget(e, pathSeg)
	e.CopiedFrom(makeJsonRef(fromSeg))
	return []*Entry{e}, nil
}

// samePContainerSegs checks that from and path share the same parent and returns
// the parent path, the from last segment, and the path last segment.
func samePContainerSegs(fromSegs, pathSegs []any) (Path, any, any, error) {
	if len(fromSegs) == 0 || len(pathSegs) == 0 {
		return P, nil, nil, fmt.Errorf("path must not be root")
	}
	fromParent := fromSegs[:len(fromSegs)-1]
	pathParent := pathSegs[:len(pathSegs)-1]
	if len(fromParent) != len(pathParent) {
		return P, nil, nil, fmt.Errorf("cross-container operations are not supported")
	}
	for i := range fromParent {
		if fromParent[i] != pathParent[i] {
			return P, nil, nil, fmt.Errorf("cross-container operations are not supported")
		}
	}
	return buildDeltaPath(fromParent), fromSegs[len(fromSegs)-1], pathSegs[len(pathSegs)-1], nil
}

func appendJsonTarget(e *Entry, seg any) {
	switch s := seg.(type) {
	case int:
		e.AppendTargets(target.Indices(s))
	case string:
		e.AppendTargets(target.StringKeys(s))
	}
}

func makeJsonRef(seg any) ref.Ref {
	switch s := seg.(type) {
	case int:
		return ref.Index(s)
	case string:
		return ref.StringKey(s)
	default:
		return ref.Ref{}
	}
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

func buildDeltaPath(segs []any) Path {
	p := P
	for _, seg := range segs {
		switch s := seg.(type) {
		case int:
			p = p.I(s)
		case string:
			p = p.S(s)
		}
	}
	return p
}

// jsonToBytes converts a JSON scalar value to Delta-encoded bytes.
// Strings are stored as raw UTF-8. Booleans and integers use varint encoding.
// Floating-point numbers use fixed64 encoding. Null produces nil.
func jsonToBytes(v json.RawMessage) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	switch v[0] {
	case '"':
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, err
		}
		return String(s), nil
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return nil, err
		}
		return Bool(b), nil
	case 'n':
		return nil, nil
	default:
		var i int64
		if err := json.Unmarshal(v, &i); err == nil {
			return Int(i), nil
		}
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			return Double(f), nil
		}
		return nil, fmt.Errorf("unsupported JSON value: %s", v)
	}
}
