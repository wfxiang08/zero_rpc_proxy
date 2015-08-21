package queue

import (
	"container/heap"
	"fmt"
	"github.com/wfxiang08/rpc_proxy/utils/log"
	"time"
)

const (
	INVALID_INDEX = -1
)

// An Item is something we manage in a priority queue.
type Worker struct {
	Identity string    // Heap中的Item对应的value
	priority int       // 元素优先级
	index    int       // 在Heap中的位置，-1表示不在heap中
	Expire   time.Time // Worker的过期时间
}

// 构建一个Worker
func NewWorker(identity string, slots int, expire time.Duration) *Worker {
	return &Worker{
		Identity: identity,
		priority: 0,
		index:    INVALID_INDEX,
		Expire:   time.Now().Add(expire),
	}
}

type PriorityQueue []*Worker

// 1. 实现sort接口
func (pq PriorityQueue) Len() int { return len(pq) }

// 最大优先级队列
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

// 交换两个元素的位置
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// 2. 实现Push接口
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Worker)
	item.index = n

	// 将Item放在队列的最后面
	*pq = append(*pq, item)
}

// 3. 接口实现: 删除最后一个元素，并且返回
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(*pq)

	item := old[n-1]
	item.index = INVALID_INDEX // for safety

	*pq = old[0 : n-1]
	return item
}

// 更新优先级队列中的元素
func (pq *PriorityQueue) UpdateItem(item *Worker, priority int) {
	item.priority = priority

	heap.Fix(pq, item.index)
}

func (pq *PriorityQueue) HasNextWorker() bool {
	now := time.Now()

	for pq.Len() > 0 {
		result := (*pq)[0]

		if result.index != INVALID_INDEX && result.Expire.After(now) {
			return true
		} else {
			heap.Remove(pq, result.index)
		}
	}
	return false
}

//
// 获取下一个可用的Worker
//
func (pq *PriorityQueue) NextWorker() *Worker {
	now := time.Now()
	for pq.Len() > 0 {
		result := (*pq)[0]

		if result.index != INVALID_INDEX && result.Expire.After(now) {
			// 只要活着，就留在优先级队列中，等待分配任务
			//			log.Println("Find Valid Worker...")

			result.priority -= 1

			// 调整Worker的优先级
			heap.Fix(pq, result.index)

			return result
		} else {
			log.Println("Worker Expired")
			// 只有过期的元素才删除
			heap.Remove(pq, result.index)
		}
	}

	log.Println("Has Not Worker...")
	return nil

}

func main() {
	// Some items and their priorities.
	items := map[string]int{
		"banana": 3, "apple": 2, "pear": 4,
	}

	// Create a priority queue, put the items in it, and
	// establish the priority queue (heap) invariants.
	pq := make(PriorityQueue, len(items))
	i := 0
	for value, priority := range items {
		pq[i] = &Worker{
			Identity: value,
			priority: priority,
			index:    i,
		}
		i++
	}
	heap.Init(&pq)

	// Insert a new item and then modify its priority.
	item := NewWorker("orange", 1, 0)
	heap.Push(&pq, item)
	pq.UpdateItem(item, 5)

	// Take the items out; they arrive in decreasing priority order.
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Worker)
		fmt.Printf("%.2d:%s\n", item.priority, item.Identity)
	}
}
