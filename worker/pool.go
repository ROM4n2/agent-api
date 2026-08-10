package worker

import (
	"agent-api/store"
	"context"
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
}

func NewPool(size int, store *store.Store) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	p := Pool{
		size:   size,
		store:  store,
		queue:  make(chan string, size),
		ctx:    ctx,
		cancel: cancel,
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
	for {
		select {
		case id := <-p.queue: // 有活
			p.store.Update(id, "running")

			time.Sleep(2 * time.Second)

			p.store.Update(id, "done")

		case <-p.ctx.Done(): // 收工信号
			p.wg.Done()
			return
		}
	}

}

func (p *Pool) Enqueue(id string) {
	p.queue <- id
}