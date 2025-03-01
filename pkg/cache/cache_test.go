// Copyright 2016 TiKV Project Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"sort"
	"testing"
	"time"

	. "github.com/pingcap/check"
)

func TestCore(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testRegionCacheSuite{})

type testRegionCacheSuite struct {
}

func (s *testRegionCacheSuite) TestExpireRegionCache(c *C) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache := NewIDTTL(ctx, time.Second, 2*time.Second)
	// Test Pop
	cache.PutWithTTL(9, "9", 5*time.Second)
	cache.PutWithTTL(10, "10", 5*time.Second)
	c.Assert(cache.Len(), Equals, 2)
	k, v, success := cache.pop()
	c.Assert(success, Equals, true)
	c.Assert(cache.Len(), Equals, 1)
	k2, v2, success := cache.pop()
	c.Assert(success, Equals, true)
	// we can't ensure the order which the key/value pop from cache, so we save into a map
	kvMap := map[uint64]string{
		9:  "9",
		10: "10",
	}
	expV, ok := kvMap[k.(uint64)]
	c.Assert(ok, Equals, true)
	c.Assert(expV, Equals, v.(string))
	expV, ok = kvMap[k2.(uint64)]
	c.Assert(ok, Equals, true)
	c.Assert(expV, Equals, v2.(string))

	cache.PutWithTTL(11, "11", 1*time.Second)
	time.Sleep(5 * time.Second)
	k, v, success = cache.pop()
	c.Assert(success, Equals, false)
	c.Assert(k, IsNil)
	c.Assert(v, IsNil)

	// Test Get
	cache.PutWithTTL(1, 1, 1*time.Second)
	cache.PutWithTTL(2, "v2", 5*time.Second)
	cache.PutWithTTL(3, 3.0, 5*time.Second)

	value, ok := cache.Get(1)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, 1)

	value, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, "v2")

	value, ok = cache.Get(3)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, 3.0)

	c.Assert(cache.Len(), Equals, 3)

	c.Assert(sortIDs(cache.GetAllID()), DeepEquals, []uint64{1, 2, 3})

	time.Sleep(2 * time.Second)

	value, ok = cache.Get(1)
	c.Assert(ok, IsFalse)
	c.Assert(value, IsNil)

	value, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, "v2")

	value, ok = cache.Get(3)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, 3.0)

	c.Assert(cache.Len(), Equals, 2)
	c.Assert(sortIDs(cache.GetAllID()), DeepEquals, []uint64{2, 3})

	cache.Remove(2)

	value, ok = cache.Get(2)
	c.Assert(ok, IsFalse)
	c.Assert(value, IsNil)

	value, ok = cache.Get(3)
	c.Assert(ok, IsTrue)
	c.Assert(value, Equals, 3.0)

	c.Assert(cache.Len(), Equals, 1)
	c.Assert(sortIDs(cache.GetAllID()), DeepEquals, []uint64{3})
}

func sortIDs(ids []uint64) []uint64 {
	ids = append(ids[:0:0], ids...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *testRegionCacheSuite) TestLRUCache(c *C) {
	cache := newLRU(3)

	cache.Put(1, "1")
	cache.Put(2, "2")
	cache.Put(3, "3")

	val, ok := cache.Get(3)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "3")

	val, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "2")

	val, ok = cache.Get(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	c.Assert(cache.Len(), Equals, 3)

	cache.Put(4, "4")

	c.Assert(cache.Len(), Equals, 3)

	val, ok = cache.Get(3)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	val, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "2")

	val, ok = cache.Get(4)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "4")

	c.Assert(cache.Len(), Equals, 3)

	val, ok = cache.Peek(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	elems := cache.Elems()
	c.Assert(elems, HasLen, 3)
	c.Assert(elems[0].Value, DeepEquals, "4")
	c.Assert(elems[1].Value, DeepEquals, "2")
	c.Assert(elems[2].Value, DeepEquals, "1")

	cache.Remove(1)
	cache.Remove(2)
	cache.Remove(4)

	c.Assert(cache.Len(), Equals, 0)

	val, ok = cache.Get(1)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(2)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(3)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(4)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)
}

func (s *testRegionCacheSuite) TestFifoCache(c *C) {
	cache := NewFIFO(3)
	cache.Put(1, "1")
	cache.Put(2, "2")
	cache.Put(3, "3")
	c.Assert(cache.Len(), Equals, 3)

	cache.Put(4, "4")
	c.Assert(cache.Len(), Equals, 3)

	elems := cache.Elems()
	c.Assert(elems, HasLen, 3)
	c.Assert(elems[0].Value, DeepEquals, "2")
	c.Assert(elems[1].Value, DeepEquals, "3")
	c.Assert(elems[2].Value, DeepEquals, "4")

	elems = cache.FromElems(3)
	c.Assert(elems, HasLen, 1)
	c.Assert(elems[0].Value, DeepEquals, "4")

	cache.Remove()
	cache.Remove()
	cache.Remove()
	c.Assert(cache.Len(), Equals, 0)
}

func (s *testRegionCacheSuite) TestTwoQueueCache(c *C) {
	cache := newTwoQueue(3)
	cache.Put(1, "1")
	cache.Put(2, "2")
	cache.Put(3, "3")

	val, ok := cache.Get(3)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "3")

	val, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "2")

	val, ok = cache.Get(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	c.Assert(cache.Len(), Equals, 3)

	cache.Put(4, "4")

	c.Assert(cache.Len(), Equals, 3)

	val, ok = cache.Get(3)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	val, ok = cache.Get(2)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "2")

	val, ok = cache.Get(4)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "4")

	c.Assert(cache.Len(), Equals, 3)

	val, ok = cache.Peek(1)
	c.Assert(ok, IsTrue)
	c.Assert(val, DeepEquals, "1")

	elems := cache.Elems()
	c.Assert(elems, HasLen, 3)
	c.Assert(elems[0].Value, DeepEquals, "4")
	c.Assert(elems[1].Value, DeepEquals, "2")
	c.Assert(elems[2].Value, DeepEquals, "1")

	cache.Remove(1)
	cache.Remove(2)
	cache.Remove(4)

	c.Assert(cache.Len(), Equals, 0)

	val, ok = cache.Get(1)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(2)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(3)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)

	val, ok = cache.Get(4)
	c.Assert(ok, IsFalse)
	c.Assert(val, IsNil)
}

var _ PriorityQueueItem = PriorityQueueItemTest(0)

type PriorityQueueItemTest uint64

func (pq PriorityQueueItemTest) ID() uint64 {
	return uint64(pq)
}

func (s *testRegionCacheSuite) TestPriorityQueue(c *C) {
	testData := []PriorityQueueItemTest{0, 1, 2, 3, 4, 5}
	pq := NewPriorityQueue(0)
	c.Assert(pq.Put(1, testData[1]), IsFalse)

	// it will have priority-value pair as 1-1 2-2 3-3
	pq = NewPriorityQueue(3)
	c.Assert(pq.Put(1, testData[1]), IsTrue)
	c.Assert(pq.Put(2, testData[2]), IsTrue)
	c.Assert(pq.Put(3, testData[4]), IsTrue)
	c.Assert(pq.Put(5, testData[4]), IsTrue)
	c.Assert(pq.Put(5, testData[5]), IsFalse)
	c.Assert(pq.Put(3, testData[3]), IsTrue)
	c.Assert(pq.Put(3, testData[3]), IsTrue)
	c.Assert(pq.Get(4), IsNil)
	c.Assert(pq.Len(), Equals, 3)

	// case1 test getAll ,the highest element should be the first
	entries := pq.Elems()
	c.Assert(len(entries), Equals, 3)
	c.Assert(entries[0].Priority, Equals, 1)
	c.Assert(entries[0].Value, Equals, testData[1])
	c.Assert(entries[1].Priority, Equals, 2)
	c.Assert(entries[1].Value, Equals, testData[2])
	c.Assert(entries[2].Priority, Equals, 3)
	c.Assert(entries[2].Value, Equals, testData[3])

	// case2 test remove the high element, and the second element should be the first
	pq.Remove(uint64(1))
	c.Assert(pq.Get(1), IsNil)
	c.Assert(pq.Len(), Equals, 2)
	entry := pq.Peek()
	c.Assert(entry.Priority, Equals, 2)
	c.Assert(entry.Value, Equals, testData[2])

	// case3 update 3's priority to highest
	pq.Put(-1, testData[3])
	entry = pq.Peek()
	c.Assert(entry.Priority, Equals, -1)
	c.Assert(entry.Value, Equals, testData[3])
	pq.Remove(entry.Value.ID())
	c.Assert(pq.Peek().Value, Equals, testData[2])
	c.Assert(pq.Len(), Equals, 1)

	// case4 remove all element
	pq.Remove(uint64(2))
	c.Assert(pq.Len(), Equals, 0)
	c.Assert(len(pq.items), Equals, 0)
	c.Assert(pq.Peek(), IsNil)
	c.Assert(pq.Tail(), IsNil)
}
