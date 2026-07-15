# protobuf-diff

A diff and patch library for Protocol Buffer messages. It computes the
difference between two messages as a `Delta` — itself a serializable protobuf
message — and applies it to any compatible `proto.Message`.

The library is split into two packages:

- **`dpb`** — the `Delta` / `Entry` / `Value` types and the builder helpers used
  to construct them (`NewDelta`, `Val*`, `Seg*`, `Field`, `PathOf`, …).
- **`patchproto`** — `Diff`, `Patch`, and `Patched` over `proto.Message`.

## Installation

```bash
go get github.com/lesomnus/protobuf-diff
```

## Quick start

```go
import (
	"github.com/lesomnus/protobuf-diff/dpb"
	"github.com/lesomnus/protobuf-diff/patchproto"
)

// Compute a Delta that turns `from` into `to`.
delta, err := patchproto.Diff(from, to)

// Apply it to a clone (or use Patch to mutate in place).
got, err := patchproto.Patched(from, delta)
// proto.Equal(got, to) == true
```

## Concepts

### Delta and Entry

A `Delta` is an ordered list of `Entry` operations applied in sequence. Each
`Entry` has three parts:

- **path** *(optional)* — a sequence of field segments to navigate **into** a
  nested container before operating (see [Addressing](#addressing)).
- **targets** *(optional)* — the fields, indices, or map keys **within** the
  reached container to operate on. With **no targets**, the op applies to the
  reached container itself — see [Root operations](#root-operations).
- **kind** — exactly one operation: `remove`, `test`, `insert`, `assign`,
  `move`, `copy`, or `nest`.

`Delta`, `Entry`, and `Value` are ordinary protobuf messages, so a delta can be
serialized, stored, and transmitted like any other message.

### Computing a Delta

`patchproto.Diff` compares two messages and produces a `Delta` that transforms
`from` into `to`.

```go
delta, err := patchproto.Diff(from, to)
```

The diff walks every field recursively:

- **Scalar fields** — `assign` the new value when it changes, or `remove` when
  the field is cleared.
- **Singular message fields** — recurse and wrap the result in a `nest`; if the
  field is newly set, `assign` the whole message value.
- **Repeated fields** — a `nest` addressing list indices: `assign` (scalar
  element) or a recursive `nest` (message element) for changed elements,
  `remove` for trailing elements that disappear, and `insert` (at index `-1`) to
  append new elements.
- **Map fields** — a `nest` addressing map keys: `remove` for deleted keys,
  `assign` for new or changed scalar values, and `nest` for changed message
  values.

### Applying a patch

```go
// Patch in place.
err := patchproto.Patch(msg, delta)

// Clone, then patch.
patched, err := patchproto.Patched(msg, delta)
```

Both accept options such as [`WithTypes`](#message-type-resolution) and
[`WithHook`](#observing-changes).

### Building a Delta by hand

```go
e := &dpb.Entry{}
e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1)), dpb.SegField(dpb.FieldNum(2))})
e.SetAssign(dpb.ValS("hello"))

delta := dpb.NewDelta(e)
```

---

## Addressing

### Targets

Targets select what an op acts on inside the reached container. Build them with
the `dpb.Seg*` helpers and pass a slice to `entry.SetTargets`.

| Target                     | Constructor                                        |
| -------------------------- | -------------------------------------------------- |
| Message field (by number)  | `dpb.SegField(dpb.FieldNum(n))`                    |
| Message field (by name)    | `dpb.SegName("field_name")`                        |
| List index                 | `dpb.SegIndex(i)` — negative indices count from end |
| String map key             | `dpb.SegName("key")`                               |
| Integer map key            | `dpb.SegIndex(k)`                                  |

A single entry can carry multiple targets; the op is applied to each.

### Path

A `path` navigates into a nested container before the op runs. Build it from
field segments (`dpb.Field` by name, `dpb.FieldNum` by number) with
`dpb.PathOf`, and set it with `entry.SetPath`.

```go
e := &dpb.Entry{}
e.SetPath(dpb.PathOf(dpb.Field("profile"), dpb.Field("address")))
e.SetTargets([]*dpb.Segment{dpb.SegName("city")})
e.SetAssign(dpb.ValS("Seoul"))
// profile.address.city = "Seoul"
```

Within a path, a numeric segment addresses a list index or a message field
number; a name segment addresses a field name or a string map key. The container
reached by a **mutating** path must already exist — see
[Root operations](#root-operations).

### Values

`assign`, `insert`, and `test` carry a `Value`, built with the `dpb.Val*` helper
matching the target's proto type.

| Field type                                   | Helper           |
| -------------------------------------------- | ---------------- |
| `string`                                     | `dpb.ValS(v)`    |
| `bytes`                                       | `dpb.ValX(v)`    |
| `bool`                                        | `dpb.ValB(v)`    |
| `int32/64`, `sint32/64`, `sfixed32/64`        | `dpb.ValI(v)`    |
| `uint32/64`, `fixed32/64`                     | `dpb.ValU(v)`    |
| `float`, `double`                             | `dpb.ValF(v)`    |
| `enum`                                        | `dpb.ValU(number)` |
| message                                      | `dpb.ValM(m)`    |
| repeated                                     | `dpb.ValL(elems...)` |
| clear / null                                 | `dpb.ValNull()`  |

A `Value` whose kind does not match the target field's kind is rejected with an
error (never a panic).

---

## Operations

Set exactly one kind on an `Entry`. Unless noted, an op applies to every target.

### `remove`

Clears the target field, list element(s), or map key(s). Removing list elements
shrinks the list.

```go
// Message field
e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1))})
e.SetRemove(true)

// List elements (negative indices supported)
e.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(-1)})
e.SetRemove(true)

// Map keys
e.SetTargets([]*dpb.Segment{dpb.SegName("foo"), dpb.SegName("bar")})
e.SetRemove(true)
```

### `test`

Verifies that the target equals the given value; returns an error if it does
not. Use it as a precondition guard.

```go
e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(3))})
e.SetTest(dpb.ValI(42))
```

### `assign`

Sets the target to a literal value.

```go
e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(3))})
e.SetAssign(dpb.ValI(42))
```

On a **repeated** field's indices, `assign` overwrites the elements in place.

### `insert`

Creates a value without overwriting an existing one:

- **Message field** — sets the field only if it is currently absent (for fields
  that track presence; a plain proto3 scalar, which has no presence, is always
  set).
- **List** — inserts before each target index (index `-1` appends; more negative
  values count back from the end + 1).
- **Map** — sets the key only if it is not already present.

```go
// ["foo","bar","baz"] → insert "z" before index 0 and 2 → ["z","foo","bar","z","baz"]
e.SetTargets([]*dpb.Segment{dpb.SegIndex(0), dpb.SegIndex(2)})
e.SetInsert(dpb.ValS("z"))
```

### `move`

Moves the value from a source location to the target(s), clearing the source.
The source is a `FieldSegment`: a field for messages, an index for lists, a key
for maps.

```go
// Move field 2 → fields 3 and 4, clearing field 2
e.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(3)), dpb.SegField(dpb.FieldNum(4))})
e.SetMove(dpb.FieldNum(2))
```

### `copy`

Like `move`, but the source is left unchanged.

```go
// Copy map["A"] → map["B"] and map["D"]
e.SetTargets([]*dpb.Segment{dpb.SegName("B"), dpb.SegName("D")})
e.SetCopy(dpb.Field("A"))
```

For `move` and `copy`, compatible numeric conversions (e.g. between integer
widths and to/from floats) and `string` ↔ `bytes` are applied; a same-kind
move/copy always works, and an unsupported conversion returns an error. The
source must be a scalar or message field — a repeated or map field source is
rejected.

### `nest`

Recursively applies an inner `Delta` to a sub-container. The target must be a
message, a repeated field, or a map.

```go
inner := &dpb.Entry{}
inner.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(1))})
inner.SetAssign(dpb.ValS("hello"))

outer := &dpb.Entry{}
outer.SetTargets([]*dpb.Segment{dpb.SegField(dpb.FieldNum(5))}) // field 5 is a message
outer.SetNest(dpb.NewDelta(inner))
```

When the target is a **repeated** field, the inner `Delta` addresses list
indices. When the target is a **map**, the inner `Delta` addresses map keys.
Recursing one level deeper — an inner `nest` into a list element or map value —
requires that element/value to be a message.

---

## Root operations

An `Entry` with **no targets** operates on the container it lands on — the root
message, or the message / list / map reached by its `path`. This is how a whole
container is replaced, cleared, or tested in a single entry.

### Replace a message

`assign` with a message value replaces the whole message: existing fields are
cleared, then the value's fields (scalar, message, **repeated**, and **map**) are
applied. `dpb.ReplaceWith(m)` is shorthand for an entry whose `assign` is
`dpb.ValM(m)` with no targets.

```go
// Replace the root message wholesale.
delta := dpb.NewDelta(dpb.ReplaceWith(newMsg))
patched, err := patchproto.Patched(msg, delta)
```

Set a `path` to replace a nested message instead of the root:

```go
e := dpb.ReplaceWith(newSub)
e.SetPath(dpb.PathOf(dpb.Field("profile"))) // replace field "profile" wholesale
```

### Clear, test, and insert

The other kinds work at the root of any container:

| Kind     | message root                         | list root                     | map root                          |
| -------- | ------------------------------------ | ----------------------------- | --------------------------------- |
| `assign` | replace (see above)                  | replace the whole list        | replace the whole map             |
| `remove` | clear every field                    | empty the list                | empty the map                     |
| `test`   | whole message equals the value       | whole list equals the value   | whole map equals the value        |
| `insert` | fill the message only if it is empty | append the value's elements   | add only keys not already present |

```go
// Clear the whole message.
e := &dpb.Entry{}
e.SetRemove(true)

// Replace a repeated field with a new list.
e := &dpb.Entry{}
e.SetPath(dpb.PathOf(dpb.Field("tags")))
e.SetAssign(dpb.ValL(dpb.ValS("a"), dpb.ValS("b")))

// Test that the whole message matches a value.
e := &dpb.Entry{}
e.SetTest(dpb.ValM(want))
```

A container value is built with `dpb.ValM` (message), `dpb.ValL` (list), or a
hand-built `dpb.Struct` whose `KeyValue` keys are the map keys (map). A `test`
with `dpb.ValNull()` asserts the container is empty.

Reaching a nested container via `path` for a **mutating** op (`assign`,
`insert`, `remove`, `nest`) requires that container to already be present;
otherwise the op returns an error (it does not auto-create). A read-only `test`
may still traverse an absent message, list, or map field — though not an absent
map key.

---

## JSON Patch interop

`dpb.FromJsonPatch` converts an RFC 6902 JSON Patch document into a `Delta`.

```go
import "github.com/lesomnus/protobuf-diff/jsonpatch"

doc := jsonpatch.Doc{
	{Op: "replace", Path: "/name", Value: json.RawMessage(`"world"`)},
	{Op: "remove", Path: "/tags/0"},
}
delta, err := dpb.FromJsonPatch(doc)
```

Mapping notes: integer path segments (and the `-` token) are list indices;
`add` on a list index becomes `insert`, `add` on a key and `replace` become
`assign`; `move` / `copy` are supported within the same parent container; `test`
ops are ignored.

## Observing changes

Pass `patchproto.WithHook` to be called each time a field is modified, with the
path, the before/after frames, and the entry responsible.

```go
err := patchproto.Patch(msg, delta, patchproto.WithHook(
	func(path []dpb.PathEntry, before, after dpb.Frame, e *dpb.Entry) {
		// observe (path, before, after, e)
	},
))
```

Hooks fire for field-, index-, and key-level changes and for message-root
operations; whole-list and whole-map root operations do not emit per-element
notifications.

## Message-type resolution

Applying an `assign`/`insert` whose value is a message requires resolving the
message type. By default `protoregistry.GlobalTypes` is used; pass
`patchproto.WithTypes` to supply a custom resolver.

```go
err := patchproto.Patch(msg, delta, patchproto.WithTypes(myResolver))
```

---

## Planned features

- Diff `string` and `bytes` with algorithms like Myers or Patience to generate
  minimal deltas for large text/binary fields.
- Direct patching of protobuf wire-format byte slices without deserializing to a
  `proto.Message`.
- Direct diffing of protobuf wire-format byte slices.
