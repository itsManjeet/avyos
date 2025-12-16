package job

import "sync"

type Pool struct {
	jobs chan Job
	wg   sync.WaitGroup
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
		_ = job.Run()
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
