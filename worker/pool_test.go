package worker

import (
	"agent-api/store"
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
