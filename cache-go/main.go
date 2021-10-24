package main

import (
	"container/heap"
	"container/list"
	"fmt"
	"sort"
	"time"
)

/*
  The PriorityExpiryCache uses the following data structure to manage internal data:
    * items: the underlying data of the cache
    * expiryValues: the oldest item in cache (the first to evict)
    * priorityValues: the lowest priority item in cache (the next priority to evict)
    * priorityMap: the order of use for items of same priority

  To properly remove items from the cache and from all corresponding data structures,
  we need to be able to remove from a heap, which is normally not supported. An int pointer
  is saved in the expiryIndexMap and priorityIndexMap to check before re-inserting the
  same priority and also track the index used to remove items once a priority/expiry
  does not exist in the PEC. The priorityIndexMap maps a priority to its index in the heap,
  this value is updated whenever an item changes locations in the heap by the implementation
  of the ExpiryHeap and PriorityHeap functions. The heap implementations use an array of
  item pointers but only read the item's expiry or priority value and update the index pointers.
  My reasoning for re-using the item data structure for these was to reduce data duplication
  by storing
  and to have the items store relevant data to remove an item.

  I assumes that we would have unique expiry times given the interface tracks time with the
  high precision `time.Time` type. If expiry times should not be assumed unique, I would need
  an additional expiryMap to map expiry times to a doubly linked list of items, similar to how
  priorityMap is used. This change should not affect the overall runtime.

  Suppose the PEC needed to be thread-safe. Using the maps and heaps for priority and expiry
  would clearly not be safe in this environment. To account for this, I would choose to add
  a single lock/mutex to the PEC that would be locked before underlying data structures
  were modified. This would come at a performance cost, since all heap remove, map delete
  operations would have to be performed atomically to avoid different threads reading
  invalid data or crashing by mutating the underlying data structures.


*/
type PriorityExpiryCache struct {
	maxItems int

	items map[string]*Item

	expiryValues *ExpiryHeap

	// map expiryTime -> index in ExpiryHeap
	expiryIndexMap map[time.Time]*int

	priorityValues *PriorityHeap

	// map priority -> index in PriorityHeap
	priorityIndexMap map[int]*int

	// priority -> doubly linked list of items in LRU order
	priorityMap map[int]*list.List
}

// Item should provide O(1) access to pointer in priorityMap
type Item struct {
	key      string
	value    interface{}
	priority int
	expire   time.Time

	expiryHeapIndex *int
	priorityIndex   *int

	priorityListNode *list.Element
}

// BEGIN ExpiryHeap

/*
  ExpiryHeap is created to keep track of the earliest expire time
  when removing items.

  The data structure uses a pointer to keep track of each expiry's
  index inside the structure so we can call heap.Remove() to remove
  invalid entries.
*/

type ExpiryHeap []*Item

func (h ExpiryHeap) Len() int           { return len(h) }
func (h ExpiryHeap) Less(i, j int) bool { return h[i].expire.Before(h[j].expire) }
func (h ExpiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	*h[i].expiryHeapIndex = i
	*h[j].expiryHeapIndex = j
}

func (h *ExpiryHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*Item))
	arr := *h
	n := len(arr)
	last := arr[n-1]
	*last.expiryHeapIndex = n - 1
}

func (h *ExpiryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *ExpiryHeap) Top() interface{} {
	return (*h)[0]
}

// END ExpiryHeap

// BEGIN PriorityHeap

/*
  PriorityHeap is created to keep track of the lowest priority
  when removing items.

  Reasoning is similar to ExpiryHeap and these separate implementations
  are required of course because Go does not yet support generics in the
  latest stable release.
*/

type PriorityHeap []*Item

func (h PriorityHeap) Len() int           { return len(h) }
func (h PriorityHeap) Less(i, j int) bool { return h[i].priority < h[j].priority }
func (h PriorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	*h[i].priorityIndex = i
	*h[j].priorityIndex = j
}

func (h *PriorityHeap) Push(x interface{}) {
	*h = append(*h, x.(*Item))
	arr := *h
	n := len(arr)
	last := arr[n-1]
	*last.priorityIndex = n - 1
}

func (h *PriorityHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *PriorityHeap) Top() interface{} {
	return (*h)[0]
}

// END PriorityHeap

// BEGIN PriorityExpiryCache

func NewCache(maxItems int) *PriorityExpiryCache {
	return &PriorityExpiryCache{
		maxItems:         maxItems,
		items:            make(map[string]*Item),
		priorityMap:      make(map[int]*list.List),
		priorityIndexMap: make(map[int]*int),
		expiryIndexMap:   make(map[time.Time]*int),
		expiryValues:     &ExpiryHeap{},
		priorityValues:   &PriorityHeap{},
	}
}

/*

Get(): O(1)
  The only non-trivial operation is moving the accessed item to the front of the doubly
  linked list, which is an O(1) operation to re-assign pointers. Other operations are constant as well.

*/

func (c *PriorityExpiryCache) Get(key string) interface{} {
	if item, ok := c.items[key]; ok {

		// check if key is expired
		if time.Now().After(item.expire) {
			return nil
		}

		// get key into the priorityMap map
		// move item to tail of list
		c.priorityMap[item.priority].MoveToFront(item.priorityListNode)

		return item
	} else {
		return nil
	}

}

/*

Set(): O(log(p) + log(e)), p = # unique priorities, e = # unique expiry times

  If a key is already present in the map, we remove the item (log(p) + log(e)).
  Inserting the new item into the items hash map is O(1), but inserting new expiry
  and priority values into respective heaps is log(p) + log(e).

  Total runtime is:
    key is present: (log(p) + log(e)) + (log(p) + log(e)) = O(log(p) + log(e))

    key is not present: (log(p) + log(e)) = O(log(p) + log(e))
*/

func (c *PriorityExpiryCache) Set(key string, value interface{}, priority int, expire time.Time) {
	var priorityIndex *int
	var expiryHeapIndex *int

	if ptr, ok := c.priorityIndexMap[priority]; ok {
		priorityIndex = ptr
	} else {
		priorityIndex = new(int)
	}

	if ptr, ok := c.expiryIndexMap[expire]; ok {
		expiryHeapIndex = ptr
	} else {
		expiryHeapIndex = new(int)
	}

	if old, ok := c.items[key]; ok {
		c.removeItem(old)
	}

	item := &Item{
		key:             key,
		value:           value,
		priority:        priority,
		expire:          expire,
		priorityIndex:   priorityIndex,
		expiryHeapIndex: expiryHeapIndex,
	}

	if dll, ok := c.priorityMap[priority]; ok {
		node := dll.PushFront(key)
		item.priorityListNode = node
	} else {
		ll := list.New()
		node := ll.PushFront(key)
		c.priorityMap[priority] = ll
		item.priorityListNode = node
	}

	if _, ok := c.priorityIndexMap[priority]; !ok {
		c.priorityIndexMap[priority] = priorityIndex
		heap.Push(c.priorityValues, item)
	}

	if _, ok := c.expiryIndexMap[expire]; !ok {
		c.expiryIndexMap[expire] = expiryHeapIndex
		heap.Push(c.expiryValues, item)
	}

	c.items[item.key] = item

	c.evictItems()
}

/*

SetMaxItems(): see evictItems()

*/
func (c *PriorityExpiryCache) SetMaxItems(maxItems int) {
	c.maxItems = maxItems

	c.evictItems()
}

/*

evictItems(): O( d * (log(p) + log(e)), d = difference between # items and maxItems, p = # unique priorities, e = # unique expiry times
  Unfortunately, I could not bring down the runtime to simply log(p) + log(e).

  If the PEC contains over maxItems, we will loop over most recent expiry times
  and then the lowest priority items in the heap while we are still over capacity.
  removeItem() will be called at most `d` times. Accessing the lowest priority/expiry
  items in a heap and accessing the back pointer of a doubly linked list are both O(1)
  in this case.

  Thus, runtime is O(d * (log(p) + log(e)))

*/

// evictItems will evict items from the cache to make room for new ones.
func (c *PriorityExpiryCache) evictItems() {
	// log(p) + log(e); p=# distinct priorityMap, e=# distinct expiryTimes

	if c.maxItems < len(c.items) {
		// check if there are expired entries to evict
		for time.Now().After(c.expiryValues.Top().(*Item).expire) &&
			len(c.items) > c.maxItems {
			c.removeItem(c.expiryValues.Top().(*Item))
		}
		for len(c.items) > c.maxItems {
			lowestPriority := c.priorityValues.Top().(*Item).priority
			lru := c.priorityMap[lowestPriority].Back()
			c.removeItem(c.items[lru.Value.(string)])
		}
	}
}

/*
removeItem(): O(log(p) + log(e)), p = # unique priorities, e = # unique expiry times

  removeItem deletes the pointers, key, expiry times, and priority from the PEC data structures.
  The item's expiry value is removed from the expiryValues heap, which is O(log(e)) since
  we will have at most log(e) (the height of heap data structure) swaps to restore the heap's
  ordering after removing the element from the array.

  We check if there are no more items with the same priority in the value using the priorityMap
  linked list and remove the item from the priorityValues heap. This operation is O(log(p)).
  Removing from the relevant hash maps is O(1).

  Total runtime is:
    unique priority (need to remove from priority heap):
      2 * (log(p) + log(e)) = O(log(p) + log(e))
    non-unique priority:
      1 * (log(p) + log(e)) = O(log(p) + log(e))

*/

func (c *PriorityExpiryCache) removeItem(item *Item) {

	// O(log(e)), e = # unique expiry values
	heap.Remove(c.expiryValues, *item.expiryHeapIndex)
	delete(c.expiryIndexMap, item.expire)

	c.priorityMap[item.priority].Remove(item.priorityListNode)

	// complexity of list.Len() is O(1)
	// https://golang.org/pkg/container/list/#List.Len
	if c.priorityMap[item.priority].Len() == 0 {
		// O(log(p)), p = # unique priority values
		heap.Remove(c.priorityValues, *item.priorityIndex)
		delete(c.priorityIndexMap, item.priority)
		delete(c.priorityMap, item.priority)
	}

	delete(c.items, item.key)
}

// Keys() is used for testing
func (c *PriorityExpiryCache) Keys() []string {
	keys := make([]string, 0)
	for k, _ := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// END PriorityExpiryCache

func printArr(arr []string) {
	fmt.Print("[")
	for _, e := range arr {
		fmt.Printf(" \"%v\",", e)
	}
	fmt.Print(" ]\n")
}

func assertEqual(actual []string, expected []string) {
	if len(actual) != len(expected) {
		panic("wrong size")
	}
	sort.Strings(actual)
	sort.Strings(expected)
	for i := 0; i < len(expected); i++ {
		if actual[i] != expected[i] {
			panic(fmt.Sprintf("Expected [%v]=%v, Actual [%v]=%v", i, expected[i], i, actual[i]))
		}
	}
}

// TODO(interviewee): writeup time/space complexity for each basic operation, describe and justify choices
func main() {

	// Original test
	c := NewCache(5)
	c.Set("A", 1, 5, time.Now().Add(time.Second*100))
	c.Set("B", 2, 15, time.Now().Add(time.Second*3))
	c.Set("C", 3, 5, time.Now().Add(time.Second*10))
	c.Set("D", 4, 1, time.Now().Add(time.Second*15))
	c.Set("E", 5, 5, time.Now().Add(time.Second*150))
	c.Get("C")

	c.SetMaxItems(5)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "C", "D", "E"})

	time.Sleep(5 * time.Second)

	c.SetMaxItems(4)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "C", "D", "E"})

	c.SetMaxItems(3)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "C", "E"})

	c.SetMaxItems(2)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"C", "E"})

	c.SetMaxItems(1)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"C"})

	// Test expiry

	c = NewCache(4)
	c.Set("A", 1, 5, time.Now().Add(time.Second*1))
	c.Set("B", 2, 15, time.Now().Add(time.Second*2))
	c.Set("C", 3, 5, time.Now().Add(time.Second*3))
	c.Set("D", 4, 1, time.Now().Add(time.Second*4))

	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "C", "D"})
	time.Sleep(1 * time.Second)

	c.SetMaxItems(3)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"B", "C", "D"})
	time.Sleep(1 * time.Second)

	c.SetMaxItems(2)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"C", "D"})
	time.Sleep(1 * time.Second)

	c.SetMaxItems(1)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"D"})

	// Test priority

	c = NewCache(4)
	c.Set("A", 1, 1, time.Now().Add(time.Second*1))
	c.Set("B", 2, 2, time.Now().Add(time.Second*2))
	c.Set("C", 3, 3, time.Now().Add(time.Second*3))
	c.Set("D", 4, 4, time.Now().Add(time.Second*4))

	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "C", "D"})

	c.SetMaxItems(3)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"B", "C", "D"})

	c.SetMaxItems(2)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"C", "D"})

	c.SetMaxItems(1)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"D"})

	// Test LRU

	c = NewCache(4)
	c.Set("D", 4, 1, time.Now().Add(time.Second*4))
	c.Set("C", 3, 1, time.Now().Add(time.Second*3))
	c.Set("B", 2, 1, time.Now().Add(time.Second*2))
	c.Set("A", 1, 1, time.Now().Add(time.Second*1))
	c.Get("B")
	c.Get("C")
	c.Get("A")
	c.Get("D")

	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "C", "D"})

	c.SetMaxItems(3)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "C", "D"})

	c.SetMaxItems(2)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "D"})

	c.SetMaxItems(1)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"D"})

	// Test re-insert
	c = NewCache(4)
	c.Set("D", 4, 1, time.Now().Add(time.Second*1))
	c.Set("C", 3, 1, time.Now().Add(time.Second*5))
	c.Set("B", 2, 1, time.Now().Add(time.Second*6))
	c.Set("A", 1, 1, time.Now().Add(time.Second*7))

	c.Set("D", 4, 4, time.Now().Add(time.Second*8))
	time.Sleep(1 * time.Second)

	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "C", "D"})

	c.SetMaxItems(3)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "B", "D"})

	c.SetMaxItems(2)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"A", "D"})

	c.SetMaxItems(1)
	printArr(c.Keys())
	assertEqual(c.Keys(), []string{"D"})
}

/*
You can use any language.

Your task is to implement a PriorityExpiryCache cache with a max capacity.  Specifically please fill out the data
structures on the PriorityExpiryCache object and implement the entry eviction method.

You do NOT need to implement the get or set methods.

It should support these operations:
  Get: Get the value of the key if the key exists in the cache and is not expired.
  Set: Update or insert the value of the key with a priority value and expiretime.
    Set should never ever allow more items than maxItems to be in the cache.
    When evicting we need to evict the lowest priority item(s) which are least recently used.

Example:
p5 => priority 5
e10 => expires at 10 seconds since epoch

c = NewCache(5)
c.Set("A", value=1, priority=5,  expireTime=100)
c.Set("B", value=2, priority=15, expireTime=3)
c.Set("C", value=3, priority=5,  expireTime=10)
c.Set("D", value=4, priority=1,  expireTime=15)
c.Set("E", value=5, priority=5,  expireTime=150)
c.Get("C")


// Current time = 0
c.SetMaxItems(5)
c.Keys() = ["A", "B", "C", "D", "E"]
// space for 5 keys, all 5 items are included

time.Sleep(5)

// Current time = 5
c.SetMaxItems(4)
c.Keys() = ["A", "C", "D", "E"]
// "B" is removed because it is expired.  e3 < e5

c.SetMaxItems(3)
c.Keys() = ["A", "C", "E"]
// "D" is removed because it the lowest priority
// D's expire time is irrelevant.

c.SetMaxItems(2)
c.Keys() = ["C", "E"]
// "A" is removed because it is least recently used."
// A's expire time is irrelevant.

c.SetMaxItems(1)
c.Keys() = ["C"]
// "E" is removed because C is more recently used (due to the Get("C") event).

*/
