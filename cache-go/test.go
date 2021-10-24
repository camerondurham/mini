package main

import (
	"container/heap"
	"fmt"
	"time"
)

func main() {
	expiryHeap := &ExpiryHeap{}
	priorityHeap := &PriorityHeap{}

	i1 := &Item{
		key:             "A",
		value:           1,
		priority:        6,
		expiryHeapIndex: new(int),
		priorityIndex:   new(int),
		expire:          time.Now(),
	}

	i2 := &Item{
		key:             "B",
		value:           2,
		priority:        1,
		expiryHeapIndex: new(int),
		priorityIndex:   new(int),
		expire:          time.Now(),
	}

	i3 := &Item{
		key:             "C",
		value:           3,
		priority:        3,
		expiryHeapIndex: new(int),
		priorityIndex:   new(int),
		expire:          time.Now(),
	}

	i4 := &Item{
		key:             "C",
		value:           3,
		priority:        10,
		expiryHeapIndex: new(int),
		priorityIndex:   new(int),
		expire:          time.Now(),
	}

	items := []*Item{i1, i2, i3, i4}

	for _, v := range items {
		heap.Push(expiryHeap, v)
		heap.Push(priorityHeap, v)
	}

	for _, v := range items {
		fmt.Printf("expiry: %v, priority: %v\n", v.expiryHeapIndex, v.priorityIndex)
	}

	heap.Remove(priorityHeap, *i1.priorityIndex)

	for priorityHeap.Len() > 0 {
		pri := heap.Pop(priorityHeap)
		fmt.Printf("priority: %v\n", pri.(*Item).priority)
	}
}
