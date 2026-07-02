package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

type ResourceCollector struct {
	Interval time.Duration
	PIDs     func() []int
	Events   chan model.ResourceEvent
	previous map[int]cpuPoint
	ticks    float64
}
type cpuPoint struct {
	ticks uint64
	at    time.Time
}

func NewResource(interval time.Duration, pids func() []int) *ResourceCollector {
	return &ResourceCollector{Interval: interval, PIDs: pids, Events: make(chan model.ResourceEvent, 1024), previous: map[int]cpuPoint{}, ticks: 100}
}
func (c *ResourceCollector) Run(ctx context.Context) {
	defer close(c.Events)
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sample()
		}
	}
}
func (c *ResourceCollector) sample() {
	now := time.Now()
	for _, pid := range c.PIDs() {
		e, ticks, err := readResource(pid)
		if err != nil {
			continue
		}
		if p, ok := c.previous[pid]; ok && now.Sub(p.at) > 0 {
			e.CPUPercent = float64(ticks-p.ticks) / c.ticks / now.Sub(p.at).Seconds() * 100
		}
		c.previous[pid] = cpuPoint{ticks, now}
		e.EventType = "resource_sample"
		e.Timestamp = now
		c.Events <- e
	}
}
func readResource(pid int) (model.ResourceEvent, uint64, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return model.ResourceEvent{}, 0, err
	}
	s := string(stat)
	end := strings.LastIndex(s, ")")
	if end < 0 {
		return model.ResourceEvent{}, 0, fmt.Errorf("bad stat")
	}
	comm := s[strings.Index(s, "(")+1 : end]
	f := strings.Fields(s[end+2:])
	if len(f) < 22 {
		return model.ResourceEvent{}, 0, fmt.Errorf("short stat")
	}
	ut, _ := strconv.ParseUint(f[11], 10, 64)
	st, _ := strconv.ParseUint(f[12], 10, 64)
	rssPages, _ := strconv.ParseInt(f[21], 10, 64)
	var readB, writeB uint64
	if file, err := os.Open(fmt.Sprintf("/proc/%d/io", pid)); err == nil {
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			x := strings.Fields(sc.Text())
			if len(x) == 2 && x[0] == "read_bytes:" {
				readB, _ = strconv.ParseUint(x[1], 10, 64)
			}
			if len(x) == 2 && x[0] == "write_bytes:" {
				writeB, _ = strconv.ParseUint(x[1], 10, 64)
			}
		}
		file.Close()
	}
	return model.ResourceEvent{PID: pid, Process: comm, RSSBytes: uint64(max(rssPages, 0)) * uint64(os.Getpagesize()), DiskReadBytes: readB, DiskWriteBytes: writeB}, ut + st, nil
}
