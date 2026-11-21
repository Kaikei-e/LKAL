package lkal

import "slices"

type Location struct {
	Key   Pos
	Value Pos
	Elems []Pos
}

type locNode struct {
	key      Pos
	value    Pos
	elems    []Pos
	terminal bool
	children map[string]*locNode
}

// Name returns the file name passed to Parse. A nil File yields "".
func (f *File) Name() string {
	if f == nil {
		return ""
	}
	return f.name
}

// Locate returns the source position of path. Concurrent Locate calls on the
// same File are safe. Locate must not run at the same time as Set or Delete:
// those methods rewrite the syntax tree, and an in-flight Locate can read a
// torn tree or put a stale position index back after it was cleared.
func (f *File) Locate(path []string) (Location, bool) {
	if f == nil || f.body == nil {
		return Location{}, false
	}
	if len(path) == 0 {
		return Location{Key: f.versionPos, Value: f.versionPos}, true
	}

	cur := f.getLocIndex()
	for _, seg := range path {
		cur = cur.children[seg]
		if cur == nil {
			return Location{}, false
		}
	}
	return Location{
		Key:   cur.key,
		Value: cur.value,
		Elems: slices.Clone(cur.elems),
	}, true
}

func (f *File) getLocIndex() *locNode {
	if root := f.locIndex.Load(); root != nil {
		return root
	}
	root := &locNode{}
	buildLocBlock(root, f.body)
	if f.locIndex.CompareAndSwap(nil, root) {
		return root
	}
	if existing := f.locIndex.Load(); existing != nil {
		return existing
	}
	return root
}

func ensureIntermediate(cur *locNode, text string, keyPos, valPos Pos) (*locNode, bool) {
	if cur.children == nil {
		cur.children = make(map[string]*locNode)
	}
	if child, ok := cur.children[text]; ok {
		if child.terminal {
			return nil, false
		}
		return child, true
	}
	child := &locNode{key: keyPos, value: valPos}
	cur.children[text] = child
	return child, true
}

func buildLocBlock(parent *locNode, b *block) {
	if b == nil || len(b.entries) == 0 {
		return
	}
	if parent.children == nil {
		distinctHint := 0
		for _, e := range b.entries {
			if len(e.labels) == 0 {
				distinctHint++
			}
		}
		if distinctHint > 0 {
			parent.children = make(map[string]*locNode, distinctHint)
		} else {
			parent.children = make(map[string]*locNode)
		}
	}
	for _, e := range b.entries {
		cur := parent
		lastText := e.key
		lastPos := e.keyPos
		if len(e.labels) > 0 {
			var ok bool
			cur, ok = ensureIntermediate(cur, e.key, e.keyPos, e.labels[0].pos)
			if !ok {
				continue
			}
			for i := 0; i < len(e.labels)-1; i++ {
				cur, ok = ensureIntermediate(cur, e.labels[i].text, e.labels[i].pos, e.labels[i+1].pos)
				if !ok {
					break
				}
			}
			if !ok {
				continue
			}
			last := e.labels[len(e.labels)-1]
			lastText = last.text
			lastPos = last.pos
		}

		if cur.children == nil {
			cur.children = make(map[string]*locNode)
		}
		if _, exists := cur.children[lastText]; exists {
			continue
		}

		child := &locNode{
			key:      lastPos,
			terminal: true,
		}
		if e.kind == entryRecord {
			if e.body != nil {
				child.value = e.body.lbrace
				buildLocBlock(child, e.body)
			}
		} else if e.value != nil {
			child.value = e.value.nodePos()
			if lst, ok := e.value.(*listNode); ok && len(lst.elems) > 0 {
				elems := make([]Pos, len(lst.elems))
				for i, el := range lst.elems {
					elems[i] = el.tok.pos
				}
				child.elems = elems
			}
		}
		cur.children[lastText] = child
	}
}
