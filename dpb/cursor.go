package dpb

type Cursor struct {
	Entries []PathEntry
	Hooks   []func(p []PathEntry, entry *Entry)
}

func (c *Cursor) Push(e PathEntry) {
	c.Entries = append(c.Entries, e)
}

func (c *Cursor) Pop() {
	c.Entries = c.Entries[:len(c.Entries)-1]
}

func (c *Cursor) Notify(entry *Entry) {
	for _, h := range c.Hooks {
		h(c.Entries, entry)
	}
}
