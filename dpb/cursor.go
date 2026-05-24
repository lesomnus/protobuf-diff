package dpb

type Cursor struct {
	Entries []PathEntry
	Hooks   []func(p []PathEntry, before, after Frame, entry *Entry)
}

func (c *Cursor) Push(e PathEntry) {
	c.Entries = append(c.Entries, e)
}

func (c *Cursor) Pop() {
	c.Entries = c.Entries[:len(c.Entries)-1]
}

func (c *Cursor) Notify(before, after Frame, entry *Entry) {
	for _, h := range c.Hooks {
		h(c.Entries, before, after, entry)
	}
}

type Frame interface {
	// Print value at the cursor path.
	String() string
}
