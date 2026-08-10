package worker

import (
	"agent-api/store"
	"strconv"
	"testing"
	"time"
)

func TestWorker_ProcessesTask(t *testing.T) {
	s := store.NewStore()
	p := NewPool(3, s)
	p.Start()
	defer p.Stop()

	id := s.Create("hello")
	p.queue <- id

	deadline := time.Now().Add(time.Second) // 最多等 1s
	for {
		task, _ := s.Get(id)
		if task.Status == "running" {
			break // 等到 running，出循环
		}
		if time.Now().After(deadline) {
			t.Fatal("1s 内没变成 running") // 超时 = 失败
		}
		time.Sleep(10 * time.Millisecond) // 喘口气再查
	}

	deadline = time.Now().Add(4 * time.Second) // 最多等 4s
	for {
		task, _ := s.Get(id)
		if task.Status == "done" {
			break // 等到 done
		}
		if time.Now().After(deadline) {
			t.Fatal("4s 内没变成 done") // 超时 = 失败
		}
		time.Sleep(10 * time.Millisecond) // 喘口气再查
	}

}

func TestPool_LimitsConcurrency(t *testing.T) {
	s := store.NewStore()
	p := NewPool(3, s)
	p.Start()
	defer p.Stop()

	// Create 6 个任务，ID 存进 ids（名字用 strconv.Itoa 拼：s.Create("task-" + strconv.Itoa(i))）
	// 全塞进 p.queue
	var ids []string
	for i := 0; i < 6; i++ {
		ids = append(ids, s.Create("task-"+strconv.Itoa(i)))
	}
	for _, id := range ids {
		p.queue <- id
	}

	// 轮询（1s deadline）：等到 running 数量 == 3
	//   统计 running 数：for 循环遍历 ids，s.Get(id) 数 status == "running" 的
	deadline := time.Now().Add(time.Second)
	for {
		running := 0
		for _, id := range ids { // 遍历所有任务数一下
			task, _ := s.Get(id)
			if task.Status == "running" {
				running++
			}
		}
		if running == 3 {
			break
		} // 3 个在跑 = 条件满足
		if time.Now().After(deadline) {
			t.Fatal("1s 内没到 3 个 running")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 断言：此时 pending 数量 == 3（遍历数 status == "pending"）
	pending := 0
	for _, id := range ids {
		task, _ := s.Get(id)
		if task.Status == "pending" {
			pending++
		}
	}
	if pending != 3 {
		t.Errorf("pending 数量 %d, want 3", pending)
	}

	// 轮询（5s deadline）：等到全部 6 个 done（为什么 5s？两批各 2s，第二批 ~4s 完）
	deadline = time.Now().Add(5 * time.Second)
	for {
		done := 0
		for _, id := range ids {
			task, _ := s.Get(id)
			if task.Status == "done" {
				done++
			}
		}
		if done == 6 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("5s 内没全 done")
		}
		time.Sleep(10 * time.Millisecond)
	}

}
