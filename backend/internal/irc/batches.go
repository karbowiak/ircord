package irc

import "github.com/lrstanley/girc"

type multiline struct {
	historyRef string
	historical bool
	first      girc.Event
	text       string
	lines      int
}

func (c *connection) batch(e girc.Event) {
	if len(e.Params) == 0 || len(e.Params[0]) < 2 {
		return
	}
	ref := e.Params[0][1:]
	c.mu.Lock()
	if e.Params[0][0] == '+' {
		if len(e.Params) >= 2 && e.Params[1] == "draft/multiline" && len(c.batches) < 32 {
			parent := e.Tags["batch"]
			c.batches[ref] = &multiline{first: e, historical: c.historyBatches[parent], historyRef: parent}
		}
		if len(e.Params) >= 2 && e.Params[1] == "chathistory" {
			c.historyBatches[ref] = true
		}
		c.openMetadataSync(ref, e)
		c.mu.Unlock()
		return
	}
	b := c.batches[ref]
	delete(c.batches, ref)
	delete(c.historyBatches, ref)
	sync := c.metadataSyncs[ref]
	delete(c.metadataSyncs, ref)
	c.mu.Unlock()
	if sync != nil {
		c.finishMetadataSync(sync)
	}
	if b != nil && b.lines > 0 {
		b.first.Params[len(b.first.Params)-1] = b.text
		delete(b.first.Tags, "batch")
		c.historyItem(b.historyRef, b.first)
		c.messageWithHistory(b.first, b.historical, b.historyRef)
	}
}

func (c *connection) receiveMultiline(e girc.Event) bool {
	if batch := e.Tags["batch"]; batch != "" {
		c.mu.Lock()
		b := c.batches[batch]
		if b != nil {
			if b.lines == 0 {
				opening := b.first
				b.first = *e.Copy()
				if b.first.Tags == nil {
					b.first.Tags = girc.Tags{}
				}
				for key, value := range opening.Tags {
					if _, ok := b.first.Tags[key]; !ok {
						b.first.Tags[key] = value
					}
				}
			} else if _, concat := e.Tags["draft/multiline-concat"]; !concat {
				b.text += "\n"
			}
			b.lines++
			b.text += e.Last()
			if len(b.text) > 16384 || b.lines > 100 {
				delete(c.batches, batch)
			}
			c.mu.Unlock()
			return true
		}
		c.mu.Unlock()
	}
	return false
}

func (c *connection) eventLabel(e girc.Event) string {
	if label := e.Tags["label"]; label != "" {
		return label
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.batchLabels[e.Tags["batch"]]
}
