package store

import (
	"encoding/json"
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
	Result json.RawMessage
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

func (s *Store) Update(ID string, status string) {
	s.Lock()
	defer s.Unlock()

	task := s.tasks[ID]
	task.Status = status
	s.tasks[ID] = task
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
