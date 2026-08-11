package worker

import (
	"agent-api/llm"
	"agent-api/store"
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type Pool struct {
	// queue 小写为私有 需要公开方法提交
	queue  chan string
	store  *store.Store
	size   int
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	llm    *llm.Client
}

func NewPool(size int, s *store.Store, client *llm.Client) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	p := Pool{
		size:   size,
		store:  s,
		queue:  make(chan string, size),
		ctx:    ctx,
		cancel: cancel,
		llm:    client,
	}
	return &p
}

func (p *Pool) Start() {
	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case id := <-p.queue:
			p.process(id)

		case <-p.ctx.Done():
			return
		}
	}
}

// process 执行单个任务：取 prompt → 调 LLM → 写回结果。
func (p *Pool) process(id string) {
	// 1. Update 成 running，检查 error（ErrNotFound → log + return）
	err := p.store.Update(id, "running")
	if err != nil {
		log.Printf("worker: update task %s to running: %v", id, err)
		return
	}

	// 2. Get 取出 task，拿 task.Prompt
	task, err := p.store.Get(id)
	if err != nil {
		log.Printf("worker: get task %s: %v", id, err)
		return
	}

	// 从 p.ctx 派生，服务关闭时在飞的请求会被一起取消
	taskCtx, cancel := context.WithTimeout(p.ctx, 60*time.Second)
	defer cancel()

	// 4. p.llm.Chat(taskCtx, task.Prompt)
	res, err := p.llm.Chat(taskCtx, task.Prompt)

	// 5. 失败：log 全文 + Complete(id, "", 脱敏error)
	if err != nil {
		log.Printf("worker: task %s failed: %v", id, err)

		if err := p.store.Complete(id, "", errors.New("upstream error")); err != nil {
			log.Printf("worker: task %s complete failed: %v", id, err)
		}
		return
	}

	//    成功：Complete(id, result, nil)
	if err := p.store.Complete(id, res, nil); err != nil {
		log.Printf("worker: task %s complete failed: %v", id, err)
	}
}

func (p *Pool) Enqueue(id string) {
	p.queue <- id
}
