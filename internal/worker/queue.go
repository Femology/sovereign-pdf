package worker

import (
	"sync"
	"time"

	"sovereign-pdf/internal/engine"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*engine.Job
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*engine.Job)}
}

func (s *JobStore) Add(job *engine.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*engine.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *JobStore) List() []*engine.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*engine.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

func (s *JobStore) UpdateStatus(id string, status engine.JobStatus, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.ErrorMsg = errMsg
		if status == engine.StatusCompleted || status == engine.StatusFailed {
			j.CompletedAt = time.Now()
		}
	}
}
