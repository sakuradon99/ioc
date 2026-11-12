package ioc

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		expr         string
		wantValue    string
		wantOptional bool
	}{
		{expr: "", wantValue: ""},
		{expr: "name", wantValue: "name"},
		{expr: ";optional", wantValue: "", wantOptional: true},
		{expr: "name*;optional", wantValue: "name*", wantOptional: true},
		{expr: "name;other", wantValue: "name"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			injectTag := ParseInjectTag(tt.expr)
			assert.Equal(t, tt.wantValue, injectTag.Value())
			assert.Equal(t, tt.wantOptional, injectTag.Optional())

			valueTag := ParseValueTag(tt.expr)
			assert.Equal(t, tt.wantValue, valueTag.Value())
			assert.Equal(t, tt.wantOptional, valueTag.Optional())
		})
	}
}

func TestValueMapGetValue(t *testing.T) {
	m := valueMap{
		"a": map[string]any{
			"b": valueMap{"c": 1},
		},
		"scalar": "x",
	}

	assert.Equal(t, 1, m.GetValue([]string{"a", "b", "c"}))
	assert.Equal(t, "x", m.GetValue([]string{"scalar"}))
	assert.Nil(t, m.GetValue([]string{"scalar", "child"}), "cannot descend into a scalar")
	assert.Nil(t, m.GetValue([]string{"missing"}))
	assert.Nil(t, m.GetValue(nil))
}

type errProvider struct{ calls int }

func (p *errProvider) Provide() (map[string]any, error) {
	p.calls++
	if p.calls == 1 {
		return nil, assert.AnError
	}
	return map[string]any{"k": "v"}, nil
}

func TestValueManagerRetriesFailedProvider(t *testing.T) {
	m := newValueManagerImpl()
	m.AddValueProvider(mapProvider{"base": "b"})
	p := &errProvider{}
	m.AddValueProvider(p)
	m.AddValueProvider(nil)

	_, _, err := m.GetProperty("k")
	assert.ErrorIs(t, err, assert.AnError)

	v, ok, err := m.GetProperty("k")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "v", v)
	assert.Len(t, m.valueMaps, 2, "earlier providers must not be loaded twice")
}

func TestConditionExecutor(t *testing.T) {
	vm := newValueManagerImpl()
	vm.AddValueProvider(mapProvider{
		"app": map[string]any{"name": "demo", "replicas": 3},
	})
	exec := newConditionExecutorImpl(vm)

	tests := []struct {
		condition string
		want      bool
		wantErr   bool
	}{
		{condition: "", want: true},
		{condition: "#app.name == 'demo'", want: true},
		{condition: "#app.name == 'other'", want: false},
		{condition: "#app.replicas > 2 && #app.name == 'demo'", want: true},
		{condition: "#app.missing == nil", want: true},
		{condition: "1 + 1", wantErr: true},
		{condition: "((", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			got, err := exec.Execute(tt.condition)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReentrantMutex(t *testing.T) {
	t.Run("same goroutine can lock again", func(t *testing.T) {
		var m reentrantMutex
		done := make(chan struct{})
		go func() {
			m.Lock()
			m.Lock()
			m.Unlock()
			m.Unlock()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("reentrant lock blocked")
		}
	})

	t.Run("excludes other goroutines", func(t *testing.T) {
		var (
			m       reentrantMutex
			wg      sync.WaitGroup
			counter int
		)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					m.Lock()
					m.Lock()
					counter++
					m.Unlock()
					m.Unlock()
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 5000, counter)
	})
}

func TestGoroutineID(t *testing.T) {
	main := goroutineID()
	assert.Positive(t, main)

	other := make(chan int64)
	go func() { other <- goroutineID() }()
	assert.NotEqual(t, main, <-other)
}

type concurrentObj struct {
	dep *depObj `inject:""`
}

func TestConcurrentGetObject(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[depObj]()))
	require.NoError(t, c.Register(typeOf[concurrentObj]()))

	var wg sync.WaitGroup
	results := make([]any, 20)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			obj, err := c.GetObject("", typeOf[concurrentObj]())
			assert.NoError(t, err)
			results[i] = obj
		}(i)
	}
	wg.Wait()

	for _, r := range results[1:] {
		assert.Same(t, results[0], r, "singleton must be created once")
	}
}
