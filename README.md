# protobuf-diff

A diff and patch library for Protocol Buffer messages.
It computes the difference between two messages as a `Delta` — a serializable protobuf message — and applies it to any compatible `proto.Message`.

## Installation

```bash
go get github.com/lesomnus/protobuf-diff
```

## Concepts

### Delta and Entry

A `Delta` is a list of `Entry` operations applied sequentially. Each `Entry` describes:

- **targets** — the fields, indices, or map keys to operate on (can be multiple)
- **kind** — the operation to perform (`deleted`, `assigned`, `merged`, `copied`, `scattered`, `swapped`, `nested`)
- **flags** — `no_insert` / `no_update` to conditionally skip targets

### Computing a Delta

`Diff` compares two messages and produces a `Delta` that transforms `from` into `to`.

```go
delta, err := dpb.Diff(from, to)
```

The diff walks all fields recursively:

- **Scalar fields** — emits an `assigned` entry when the value changes, or `deleted` when the field is cleared.
- **Message fields** — recurses and wraps the result in a `nested` entry.
- **Repeated fields** — diffs element-by-element at the same index; trailing elements that disappear are deleted, and new elements are appended.
- **Map fields** — emits `deleted` for removed keys, `assigned` for new or changed scalar values, and `nested` for changed message values.

### Applying a patch

```go
// Patch in place
err := dpb.Patch(msg, delta)

// Clone then patch
patched, err := dpb.Patched(msg, delta)
```

### Building a Delta

```go
entry := &dpb.Entry{}
entry.AppendTargets(target.Fields(1, 2))
entry.SetAssigned(dpb.String("hello"))

delta := dpb.NewDelta(entry)
```

---

## Operations

### `deleted`

Clears the target field, list element, or map key. Unaffected by `no_insert` / `no_update`.

```go
// Message field
entry.AppendTargets(target.Fields(1))
entry.SetDeleted(true)

// List elements (supports negative indices)
entry.AppendTargets(target.Indices(0, -1))
entry.SetDeleted(true)

// Map keys
entry.AppendTargets(target.StringKeys("foo", "bar"))
entry.SetDeleted(true)
```

---

### `assigned`

Sets the target to a literal value encoded with the appropriate helper.

| Field type                                   | Helper           |
| -------------------------------------------- | ---------------- |
| `string`                                     | `dpb.String(v)`  |
| `bytes`                                      | `dpb.Bytes(v)`   |
| `bool`                                       | `dpb.Bool(v)`    |
| `int32`, `int64`, `uint32`, `uint64`         | `dpb.Int(v)`     |
| `sint32`, `sint64`                           | `dpb.Signed(v)`  |
| `fixed32`, `fixed64`, `sfixed32`, `sfixed64` | `dpb.Fixed(v)`   |
| `float`                                      | `dpb.Float(v)`   |
| `double`                                     | `dpb.Double(v)`  |
| `enum`                                       | `dpb.Enum(v)`    |
| message                                      | `dpb.Message(v)` |

```go
entry.AppendTargets(target.Fields(3))
entry.SetAssigned(dpb.Int(42))
```

**On repeated fields**, `assigned` has two modes controlled by `no_update`:

- **update mode** (default): overwrites the elements at the given indices.
- **insert mode** (`no_update=true`): inserts before the given indices. Index `-1` appends to the end; more negative values count backward from the end + 1.

```go
// ["foo","bar","baz"] → insert "z" before index 0 and 2 → ["z","foo","bar","z","baz"]
entry.SetNoUpdate(true)
entry.AppendTargets(target.Indices(0, 2))
entry.SetAssigned(dpb.String("z"))
```

---

### `merged`

Merges a partial message value into target fields using `proto.Merge`. The target field must be a message type.

```go
patch := &MyMsg{Name: "world"}
entry.AppendTargets(target.Fields(5))
entry.SetMerged(dpb.Message(patch))
```

---

### `copied`

Copies the value from a source field (or list index / map key) to one or more targets. The source is unchanged.

```go
// Copy field 2 → fields 3 and 4
entry.AppendTargets(target.Fields(3, 4))
entry.CopiedFrom(ref.Field(2))

// Copy list[1] → list[0] and list[2]
entry.AppendTargets(target.Indices(0, 2))
entry.CopiedFrom(ref.Index(1))

// Copy map["A"] → map["B"] and map["D"]
entry.AppendTargets(target.StringKeys("B", "D"))
entry.CopiedFrom(ref.StringKey("A"))
```

Numeric types (integers, floats, bool, enum) are mutually castable. `string` ↔ `bytes` are also castable.

On repeated fields, `no_update=true` switches to insert mode (same as `assigned`).

---

### `scattered`

Like `copied`, but clears the source after the copy — a **move** operation. On repeated fields, the source element is removed and the list shrinks by one.

```go
entry.AppendTargets(target.Fields(3, 4))
entry.ScatteredFrom(ref.Field(2))
```

---

### `swapped`

Swaps the value at each target with the value at the reference. For repeated fields, swaps cycle through all targets and the reference index in sequence.

```go
// Swap field 1 ↔ field 2
entry.AppendTargets(target.Fields(1))
entry.SwappedWith(ref.Field(2))

// Swap list[2] ↔ list[0]
entry.AppendTargets(target.Indices(2))
entry.SwappedWith(ref.Index(0))
```

---

### `nested`

Recursively applies an inner `Delta` to a sub-field. The target field must be a message, a repeated field, or a map.

```go
inner := &dpb.Entry{}
inner.AppendTargets(target.Fields(1))
inner.SetAssigned(dpb.String("hello"))

outer := &dpb.Entry{}
outer.AppendTargets(target.Fields(5)) // field 5 must be a message
outer.SetNested(dpb.NewDelta(inner))
```

When the target is a **repeated** field, the inner `Delta` addresses list indices. With `no_update=true` on the inner entry, a new message is constructed from the delta and inserted at the given index.

When the target is a **map** field, the inner `Delta` addresses map keys. The map value type must be a message.

---

## Flags

These flags apply to all operations except `deleted`.

| Flag        | Effect on message/map         | Effect on repeated                     |
| ----------- | ----------------------------- | -------------------------------------- |
| `no_insert` | Skip if target doesn't exist  | *(ignored)*                            |
| `no_update` | Skip if target already exists | Switch from update mode to insert mode |

```go
entry.SetNoInsert(true) // only modify existing fields/keys
entry.SetNoUpdate(true) // only create new fields/keys (or insert into list)
```

---

## Target encoding

Use the `target` package to specify one or more targets in an `Entry`.

| Target type                  | Constructor                                       |
| ---------------------------- | ------------------------------------------------- |
| Message fields               | `target.Fields(fieldNums...)`                     |
| List indices                 | `target.Indices(indices...)` — negative supported |
| String map keys              | `target.StringKeys(strings...)`                   |
| Signed integer map keys      | `target.SignedKeys(vs...)`                        |
| Fixed-width integer map keys | `target.FixedKeys(vs...)`                         |

## Reference encoding

Use the `ref` package to reference a single source location in `copied`, `scattered`, and `swapped`.

| Source type                | Constructor                                 |
| -------------------------- | ------------------------------------------- |
| Message field              | `ref.Field(fieldNum)`                       |
| List index                 | `ref.Index(i)` — negative indices supported |
| String map key             | `ref.StringKey(s)`                          |
| Signed integer map key     | `ref.SignedKey(v)`                          |
| Fixed-width 32-bit map key | `ref.Fixed32Key(v)`                         |
| Fixed-width 64-bit map key | `ref.Fixed64Key(v)`                         |


## Planned features

- Support patch for `string` and `bytes` with diff algorithems like Myers or Patience to generate minimal deltas for large text/binary fields.
- Calculate a `Delta` between two messages of the same type.
- Direct patching of protobuf wire-format byte slices without deserializing to a `proto.Message`.
- Direct diffing of protobuf wire-format byte slices.
