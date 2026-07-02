package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type JSONLogger struct {
	mu    sync.Mutex
	files map[string]*os.File
}

func New(paths ...string) (*JSONLogger, error) {
	l := &JSONLogger{files: make(map[string]*os.File)}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
			l.Close()
			return nil, err
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			l.Close()
			return nil, err
		}
		l.files[p] = f
	}
	return l, nil
}

func (l *JSONLogger) Write(path string, value any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.files[path]
	if f == nil {
		return fmt.Errorf("log path not configured: %s", path)
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func (l *JSONLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var first error
	for _, f := range l.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	l.files = map[string]*os.File{}
	return first
}

func ReadJSONLines[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []T
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for s.Scan() {
		var v T
		if json.Unmarshal(s.Bytes(), &v) == nil {
			out = append(out, v)
		}
	}
	return out, s.Err()
}
