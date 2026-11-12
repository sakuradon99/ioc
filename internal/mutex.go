package ioc

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// reentrantMutex is a mutex that can be locked again by the goroutine that already holds it.
type reentrantMutex struct {
	mu    sync.Mutex
	owner atomic.Int64
	depth int
}

func (m *reentrantMutex) Lock() {
	gid := goroutineID()
	if m.owner.Load() == gid {
		m.depth++
		return
	}
	m.mu.Lock()
	m.owner.Store(gid)
	m.depth = 1
}

func (m *reentrantMutex) Unlock() {
	m.depth--
	if m.depth == 0 {
		m.owner.Store(0)
		m.mu.Unlock()
	}
}

// goroutineID parses the current goroutine id from the stack header "goroutine 123 [running]:".
func goroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}
