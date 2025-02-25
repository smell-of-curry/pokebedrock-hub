package queue

import "container/heap"

// PriorityQueue ...
type PriorityQueue []*Entry

// Len ...
func (pq PriorityQueue) Len() int {
	return len(pq)
}

// Less ...
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].rank == pq[j].rank {
		return pq[i].joinTime.Before(pq[j].joinTime)
	}
	return pq[i].rank > pq[j].rank
}

// Swap ...
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push ...
func (pq *PriorityQueue) Push(x any) {
	entry := x.(*Entry)
	entry.index = len(*pq)
	*pq = append(*pq, entry)
}

// Pop ...
func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*pq = old[0 : n-1]
	return entry
}

// Compiler time check.
var _ heap.Interface = (*PriorityQueue)(nil)
