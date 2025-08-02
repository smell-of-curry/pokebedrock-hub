package queue

import (
	"container/heap"
	"fmt"
	"strings"
)

// PriorityQueue implements a heap.Interface for queue entries.
// It prioritizes entries by rank first, and then by join time for equal ranks.
// Higher ranks get priority over lower ranks (Admin > Trainer).
type PriorityQueue []*Entry

// Len returns the length of the priority queue.
func (pq PriorityQueue) Len() int {
	return len(pq)
}

// Less determines the ordering of entries in the priority queue.
// Returns true if the entry at index i should come before the entry at index j.
// Higher ranks come first; for equal ranks, earlier join times come first.
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].rank == pq[j].rank {
		return pq[i].joinTime.Before(pq[j].joinTime)
	}

	return pq[i].rank > pq[j].rank // Higher rank (numerically) has priority
}

// Swap swaps the entries at indices i and j and updates their indices.
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds an entry to the priority queue and updates its index.
func (pq *PriorityQueue) Push(x any) {
	entry, ok := x.(*Entry)
	if !ok {
		panic("priority queue can only push *Entry")
	}

	entry.index = len(*pq)
	*pq = append(*pq, entry)
}

// Pop removes and returns the highest-priority entry from the queue.
// The highest-priority entry is the one with highest rank or earliest join time.
func (pq *PriorityQueue) Pop() any {
	old := *pq

	n := len(old)
	if n == 0 {
		return nil
	}

	entry := old[n-1]
	old[n-1] = nil   // Avoid memory leak
	entry.index = -1 // Mark as removed
	*pq = old[0 : n-1]

	return entry
}

// String returns a string representation of the queue for debugging.
func (pq PriorityQueue) String() string {
	if len(pq) == 0 {
		return "Queue{empty}"
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("Queue{len: %d, entries: [", len(pq)))

	for i, entry := range pq {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(entry.String())
	}

	b.WriteString("]}")

	return b.String()
}

// Verify at compile time that PriorityQueue implements heap.Interface
var _ heap.Interface = (*PriorityQueue)(nil)
