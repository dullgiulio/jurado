package main

import (
	"fmt"
	"sort"
	"time"
)

type task struct {
	repeat time.Duration
	left   time.Duration
	check  *Check
}

func newTask(check *Check) (*task, error) {
	d, err := time.ParseDuration(check.Interval)
	if err != nil {
		return nil, err
	}
	return &task{
		repeat: d,
		left:   d,
		check:  check,
	}, nil
}

type taskByTimeLeft []*task

func (a taskByTimeLeft) Len() int           { return len(a) }
func (a taskByTimeLeft) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a taskByTimeLeft) Less(i, j int) bool { return a[i].left < a[j].left }

type scheduler struct {
	host string
	ts   []*task
	res  chan<- *CheckResult
	ch   chan *Check
}

func newScheduler(host string, ts []*task, res chan<- *CheckResult, nworkers int) *scheduler {
	s := &scheduler{
		host: host,
		ts:   ts,
		res:  res,
		ch:   make(chan *Check, len(ts)),
	}
	if len(ts) == 0 {
		return s
	}
	for i := 0; i < nworkers; i++ {
		go s.work()
	}
	return s
}

func (s *scheduler) work() {
	for check := range s.ch {
		fmt.Printf("executing %s\n", check)
		s.res <- check.run(s.host)
	}
}

func (s *scheduler) schedule() {
	if len(s.ts) == 0 {
		return
	}
	for {
		sort.Sort(taskByTimeLeft(s.ts))
		wait := s.ts[0].left
		for i := range s.ts {
			s.ts[i].left = -wait
			if s.ts[i].left <= 0 {
				s.ts[i].left = s.ts[i].repeat
				s.ch <- s.ts[i].check
			}
		}
		time.Sleep(wait)
	}
}
