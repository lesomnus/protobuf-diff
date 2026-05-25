package dpb

type PathEntryKind int

const (
	PathEntryField PathEntryKind = iota + 1
	PathEntryIndex
)

// PathEntry represents a human-readable segment in the path for the cursor.
type PathEntry struct {
	Kind  PathEntryKind
	Key   string
	Index int
}
