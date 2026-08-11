package store

import (
	"errors"
	"strconv"
	"sync"
)

var ErrNotFound = errors.New("task not found")

type Store struct {
	tasks map[string]Task
	sync.Mutex
	nextID int
}

type Task struct {
	ID     string
	Status string
	Prompt string
	Result string
	Error  string
}

func NewStore() *Store {
	var store Store
	store.tasks = make(map[string]Task)
	return &store
}

func (s *Store) Create(prompt string) (id string) {
	s.Lock()
	defer s.Unlock()

	s.nextID++
	id = strconv.Itoa(s.nextID)

	s.tasks[id] = Task{
		ID:     id,
		Prompt: prompt,
		Status: "pending",
	}

	return id
}

func (s *Store) Update(ID string, status string) error {
	s.Lock()
	defer s.Unlock()

	task, ok := s.tasks[ID]
	if !ok {
		return ErrNotFound
	}
	task.Status = status
	s.tasks[ID] = task

	return nil
}

func (s *Store) Complete(ID string, result string, err error) error {
	s.Lock()
	defer s.Unlock()
	task, ok := s.tasks[ID]
	if !ok {
		return ErrNotFound
	}

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		s.tasks[ID] = task

		return nil
	}

	task.Status = "done"
	task.Result = result

	s.tasks[ID] = task

	return nil
}

func (s *Store) Get(ID string) (Task, error) {
	s.Lock()
	defer s.Unlock()

	v, ok := s.tasks[ID]
	if !ok {
		return Task{}, ErrNotFound
	}
	return v, nil
}
