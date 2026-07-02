package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FlowLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
}

func NewFlowLogger(path string) (*FlowLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create flow log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("open flow log: %w", err)
	}
	return &FlowLogger{file: f, writer: bufio.NewWriterSize(f, 64*1024)}, nil
}

func (l *FlowLogger) Observe(event Event) error {
	if event.Kind != "network" {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := l.writer.Write(append(b, '\n')); err != nil {
		return err
	}
	return l.writer.Flush()
}

func (l *FlowLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.writer.Flush(); err != nil {
		_ = l.file.Close()
		return err
	}
	return l.file.Close()
}
