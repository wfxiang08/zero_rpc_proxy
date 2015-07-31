package queue

import (
	"container/heap"
	"fmt"
	color "github.com/fatih/color"
	"github.com/wfxiang08/rpc_proxy/utils/log"
	"time"
)

const (
	HEARTBEAT_LIVENESS = 3                       //  3-5 is reasonable
	HEARTBEAT_INTERVAL = 1000 * time.Millisecond //  msecs

	PPP_READY     = "\001" //  Signals worker is ready
	PPP_HEARTBEAT = "\002" //  Signals worker heartbeat
	PPP_STOP      = "\003" //  Signals worker heartbeat
	VERSION       = "\001" //  当前协议的版本

	SERVICE_STOP = -1
)

var green = color.New(color.FgGreen).SprintFunc()

// HEARTBEAT_INTERVAL * HEARTBEAT_LIVENESS
// Paranoid Pirate queue，简称PPQueue
type PPQueue struct {
	WorkerQueue PriorityQueue      // 最大优先级队列(按照slots排序)
	id2item     map[string]*Worker // 记录了Worker的信息
}

// 构建一个PPQueue
func NewPPQueue() *PPQueue {
	queue := &PPQueue{
		WorkerQueue: make(PriorityQueue, 0),
		id2item:     make(map[string]*Worker, 10),
	}
	// 初始化: PriorityQueue
	// heap.Init(&(queue.pq))

	return queue
}

//
// 获取下一个可用的Worker
// Worker的Purge也一并实现
//
func (pq *PPQueue) NextWorker() *Worker {
	return pq.WorkerQueue.NextWorker()
}

func (pq *PPQueue) HasNextWorker() bool {
	return pq.WorkerQueue.HasNextWorker()
}

func (pq *PPQueue) UpdateWorkerExpire(identity string) {
	item, ok := pq.id2item[identity]
	if ok {
		expire := HEARTBEAT_INTERVAL * HEARTBEAT_LIVENESS
		item.Expire = time.Now().Add(expire)
	}
}

//
// 更新Worker的信息，主要是更新pq.workerQueue中的priority
// 1. power == 0, 表示增加一个worker power
// 2. power > 0,  表示重新设置worker的power
// 3. power < 0,  表示对应的worker出现故障
// force: 如果为true, 则不管Worker是否存在，都会执行指定的动作
//        如果为false, 则如果Worker不存在，则直接跳过
//        STOP/READY对应force为true; HB对应的force为False
func (pq *PPQueue) UpdateWorkerStatus(identity string, power int, force bool) {

	//	fmt.Printf("UpdateWorkerStatus: identity: %s, Slots: %d\n", identity, slots)
	// 1. 维护identity <--> Item
	item, ok := pq.id2item[identity]
	expire := HEARTBEAT_INTERVAL * HEARTBEAT_LIVENESS

	if !ok {
		if power < 0 || !force {
			return
		}

		// item := 会创建一个临时变量，导致赋值失败
		item = NewWorker(identity, power, expire)
		pq.id2item[identity] = item
	} else {
		item.Expire = time.Now().Add(expire)
	}
	//	fmt.Println("Item: ", item)
	if power < 0 {
		// 下线
		if item.index == INVALID_INDEX {
			delete(pq.id2item, identity)
		} else {
			heap.Remove(&(pq.WorkerQueue), item.index)
			delete(pq.id2item, identity)
		}
		return
	}

	if power == 0 {
		// 增加一个worker slot
		item.priority += 1
	} else {
		// 开始一个新的worker
		item.priority = power
	}

	// 2. 添加到队列最末尾
	if item.index == INVALID_INDEX {
		heap.Push(&(pq.WorkerQueue), item)
	} else {
		// 重新调整了priority
		heap.Fix(&(pq.WorkerQueue), item.index)
	}
}

func (pq *PPQueue) PurgeExpired() {
	now := time.Now()
	expiredWokers := make([]*Worker, 0)
	// 给workerQueue中的所有的worker发送心跳消息
	for _, worker := range pq.WorkerQueue {
		if worker.Expire.Before(now) {
			fmt.Println("Purge Worker: ", worker.Identity, ", At Index: ", worker.index)
			expiredWokers = append(expiredWokers, worker)
		}
	}

	log.Println("expiredWokers: ", len(expiredWokers))

	// 删除过期的Worker
	for _, worker := range expiredWokers {
		log.Println("Purge Worker: ", worker.Identity, ", At Index: ", worker.index)
		heap.Remove(&(pq.WorkerQueue), worker.index)
		delete(pq.id2item, worker.Identity)
	}

	log.Println("Available Workers: ", green(fmt.Sprintf("%d", len(pq.WorkerQueue))))
}
