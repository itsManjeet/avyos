package main

import (
	"sync"

	"avyos.dev/pkg/job"
)

type udev struct {
	jobs  *job.Pool
	mutex sync.Mutex
}

func (u *udev) isIdle() bool {
	return u.jobs.IsIdle()
}
