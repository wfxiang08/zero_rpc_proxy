package queue

import (
	"container/heap"
	"github.com/wfxiang08/rpc_proxy/utils/assert"

	"testing"
)

func TestPriorityQueue(t *testing.T) {
	pq := make(PriorityQueue, 0)
	heap.Push(&pq, &Worker{Identity: "hello", priority: 10})
	heap.Push(&pq, &Worker{Identity: "h20", priority: 20})
	topItem := heap.Pop(&pq)

	t.Log("TopItem: ", topItem.(*Worker).Identity, topItem.(*Worker).priority)
	assert.Must(topItem.(*Worker).Identity == "h20")
	//	assert.Must(topItem.(*Item).value == "hello")

	topItem = heap.Pop(&pq)
	t.Log("TopItem: ", topItem.(*Worker).Identity, topItem.(*Worker).priority)

	assert.Must(true)

}
