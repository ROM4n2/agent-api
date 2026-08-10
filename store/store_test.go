package store

import (
	"testing"
)

func TestStore_Create(t *testing.T) {
	s := &Store{tasks: make(map[string]Task)}

	id := s.Create("hello")

	// 断言 1：id 非空
	if id == "" {
		t.Errorf("Create 返回的 id 是空的")
	}

	task, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get 返回的 %v", err)
	}

	// 断言 2：s.Get(id) 能取回来，Prompt == "hello"
	if task.Prompt != "hello" {
		t.Errorf("Get 返回的 Prompt 不是 hello")
	}

	// 断言 3：取回来的 Status == "pending"
	if task.Status != "pending" {
		t.Errorf("Get 返回的 Status 不是 pending")
	}
}

func TestStore_GetNotFound(t *testing.T) {
	s := &Store{tasks: make(map[string]Task)}

	_, err := s.Get("不存在的id")

	// 断言：err == ErrNotFound
	if err != ErrNotFound {
		t.Fatalf("Get 返回的 %v", err)
	}
}

func TestStore_UpdateStatus(t *testing.T) {
	s := &Store{tasks: make(map[string]Task)}

	id := s.Create("hello")
	s.Update(id, "running")

	task, err := s.Get(id)

	// 断言：err == nil
	if err != nil {
		t.Fatalf("Get 返回的 %v", err)
	}
	// 断言：task.Status == "running"
	if task.Status != "running" {
		t.Errorf("Get 返回的 Status 不为 running")
	}
}
