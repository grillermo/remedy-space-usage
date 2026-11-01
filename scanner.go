package main

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/grillermo/chicle"
)

const scanWorkers = 4

var sectionOrder = map[string]int{sectionCaches: 0, sectionArtifacts: 1, sectionLogs: 2}

type Scanner struct {
	mu       sync.Mutex
	items    []*Item
	sent     map[string]bool
	discover func() []*Item
	done     atomic.Bool

	// applied holds row keys the picker has certainly received, and accepted
	// those of the latest send. See send.
	applied  map[string]bool
	accepted map[string]bool
}

func newScanner(items []*Item, discover func() []*Item) *Scanner {
	return &Scanner{items: items, discover: discover, sent: map[string]bool{}}
}

func (s *Scanner) finished() bool { return s.done.Load() }

// rows renders the current items. A row carries its default tick only the first
// time it is sent: chicle ORs incoming ticks with its own, so repeating a tick
// would undo the user unticking that row.
func (s *Scanner) rows() []chicle.Row {
	var rows []chicle.Row
	for _, it := range s.items {
		if it.hidden() {
			continue
		}
		rows = append(rows, chicle.Row{
			Key:     it.Key,
			Section: it.Section,
			Cols:    []string{it.sizeLabel(), it.Label, it.Detail},
			Ticked:  it.Ticked && !s.sent[it.Key],
		})
		s.sent[it.Key] = true
	}
	return rows
}

func (it *Item) hidden() bool {
	return it.Measured && (it.SizeKB == 0 || it.SizeKB < it.MinSizeKB)
}

func (s *Scanner) markApplied(rows []chicle.Row) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.applied == nil {
		s.applied = map[string]bool{}
	}
	for _, r := range rows {
		s.applied[r.Key] = true
	}
}

// send delivers rows and tracks what the picker has received. The picker asks
// for the next update only after applying the last one, so an accepted send
// proves the previous rows were applied, but not these.
func (s *Scanner) send(updates chan<- []chicle.Row, rows []chicle.Row) {
	updates <- rows
	s.mu.Lock()
	previous := s.accepted
	s.accepted = map[string]bool{}
	for _, r := range rows {
		s.accepted[r.Key] = true
	}
	s.mu.Unlock()
	var applied []chicle.Row
	for key := range previous {
		applied = append(applied, chicle.Row{Key: key})
	}
	s.markApplied(applied)
}

// remember records what you could see: ticked rows as true, unticked as false.
// Rows hidden for being empty, and rows the picker never received, keep their
// earlier choice, as an absent row says nothing about what you wanted.
func (s *Scanner) remember(ticked map[string]bool, m Memory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		switch {
		case ticked[it.Key]:
			m[it.location()] = true
		case s.applied[it.Key] && !it.hidden():
			m[it.location()] = false
		}
	}
}

func (s *Scanner) sortItems() {
	sort.SliceStable(s.items, func(a, b int) bool {
		x, y := s.items[a], s.items[b]
		if x.Section != y.Section {
			return sectionOrder[x.Section] < sectionOrder[y.Section]
		}
		return x.SizeKB > y.SizeKB
	})
}

func (s *Scanner) snapshot() []chicle.Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows()
}

// start discovers and measures in the background. Rows keep their order during
// the scan so the list doesn't jump, then are sorted largest first at the end.
func (s *Scanner) start() <-chan []chicle.Row {
	updates := make(chan []chicle.Row)
	go func() {
		defer close(updates)

		found := s.discover()
		s.mu.Lock()
		s.items = append(s.items, found...)
		s.sortItems()
		pending := append([]*Item(nil), s.items...)
		s.mu.Unlock()
		s.send(updates, s.snapshot())

		work := make(chan *Item)
		var wg sync.WaitGroup
		for range scanWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for it := range work {
					kb := measure(it.SizePaths)
					s.mu.Lock()
					it.SizeKB, it.Measured = kb, true
					rows := s.rows()
					s.mu.Unlock()
					s.send(updates, rows)
				}
			}()
		}
		for _, it := range pending {
			if len(it.SizePaths) > 0 {
				work <- it
			}
		}
		close(work)
		wg.Wait()

		s.mu.Lock()
		s.sortItems()
		rows := s.rows()
		s.mu.Unlock()
		s.done.Store(true)
		s.send(updates, rows)
	}()
	return updates
}

// selected returns the items for the given row keys, in list order.
func (s *Scanner) selected(keys map[string]bool) []*Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Item
	for _, it := range s.items {
		if keys[it.Key] {
			out = append(out, it)
		}
	}
	return out
}
