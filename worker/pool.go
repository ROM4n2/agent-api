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

// Pool 用固定数量的 worker goroutine 消费任务队列，
// 以此限制同时在飞的 LLM 请求数——上游的限流和计费按并发量算。
type Pool struct {
	// 缓冲区大小等于 worker 数：满了 Enqueue 会阻塞调用方（HTTP handler），
	// 使异步退化为同步。生产上应改为 select+default 快速失败或换持久化队列。
	queue chan string
	store *store.Store
	size  int
	// ctx/cancel 是关闭信号的广播机制：Stop 一次 cancel，
	// 所有 worker 与派生出去的请求 ctx 同时收到通知。
	// 用 cancel 而非 close(queue)，避免向已关闭 channel 发送导致 panic。
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

// worker 是常驻 goroutine：循环抢队列里的任务，直到收到关闭信号。
// wg.Done 用 defer，保证 panic 退出时也不会让 Stop 永久阻塞在 Wait。
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
// 任何错误都在这里终结——worker 是 goroutine，没有调用方可以把 error 返回给谁。
func (p *Pool) process(id string) {
	// 先置 running，让轮询方立刻看到状态推进
	if err := p.store.Update(id, "running"); err != nil {
		log.Printf("worker: update task %s to running: %v", id, err)
		return
	}

	// 队列里只传了 ID，prompt 的权威副本在 store 中
	task, err := p.store.Get(id)
	if err != nil {
		log.Printf("worker: get task %s: %v", id, err)
		return
	}

	// 从 p.ctx 派生（而非 Background）：既有单次期限，
	// 又能在 Stop 时随父 ctx 一起取消在飞的请求，不必干等超时。
	taskCtx, cancel := context.WithTimeout(p.ctx, 60*time.Second)
	defer cancel()

	res, err := p.llm.Chat(taskCtx, task.Prompt)
	if err != nil {
		// 错误全文只进日志：它携带上游响应体，可能含配额、账单等内部信息。
		// task.Error 会经 GET /tasks/{id} 原样返回给调用方，因此只存粗粒度分类。
		log.Printf("worker: task %s failed: %v", id, err)

		if err := p.store.Complete(id, "", errors.New("upstream error")); err != nil {
			log.Printf("worker: task %s complete failed: %v", id, err)
		}
		return
	}

	if err := p.store.Complete(id, res, nil); err != nil {
		log.Printf("worker: task %s complete failed: %v", id, err)
	}
}

func (p *Pool) Enqueue(id string) {
	p.queue <- id
}
