package job

import (
	"sync"
	"sync/atomic"
)

type Pool struct {
	jobs   chan Job
	active int32
	wg     sync.WaitGroup
}

func NewJobPool(workersCount, queueSize int) *Pool {
	p := &Pool{
		jobs: make(chan Job, queueSize),
	}
	for i := 0; i < workersCount; i++ {
		go p.worker(i)
	}
	return p
}

func (p *Pool) worker(id int) {
	for job := range p.jobs {
		atomic.AddInt32(&p.active, 1)
		_ = job.Run()
		atomic.AddInt32(&p.active, -1)
		p.wg.Done()
	}
}

func (p *Pool) Submit(j Job) {
	p.wg.Add(1)
	p.jobs <- j
}

func (p *Pool) Shutdown() {
	p.wg.Wait()
	close(p.jobs)
}

func (p *Pool) IsIdle() bool {
	return atomic.LoadInt32(&p.active) == 0 && len(p.jobs) == 0
}

func (p *Pool) Wait() {
	p.wg.Wait()
}
