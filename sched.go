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

func schedule(host string, in chan<- *CheckResult, ts []*task) {
	if len(ts) == 0 {
		return
	}
	for {
		sort.Sort(taskByTimeLeft(ts))
		wait := ts[0].left
		for i := range ts {
			ts[i].left = -wait
			if ts[i].left <= 0 {
				ts[i].left = ts[i].repeat
				fmt.Printf("executing %s\n", ts[i].check)
				in <- ts[i].check.run(host)
			}
		}
		time.Sleep(wait)
	}
}
