package ioc

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeOf[T any]() reflect.Type { return reflect.TypeOf(new(T)).Elem() }

func withConstructor(c any) RegisterOption {
	return func(o *RegisterOptions) { o.Constructor = c }
}

type mapProvider map[string]any

func (m mapProvider) Provide() (map[string]any, error) { return m, nil }

// newStartedContainer registers the given types, starts the container and fails the test on error.
func newStartedContainer(t *testing.T, provider ValueProvider, types ...reflect.Type) *ContainerImpl {
	t.Helper()
	c := NewContainerImpl()
	if provider != nil {
		require.NoError(t, c.AddValueProvider(provider))
	}
	for _, rtp := range types {
		require.NoError(t, c.Register(rtp))
	}
	require.NoError(t, c.Start(context.Background()))
	return c
}

// ---------------------------------------------------------------------------
// Regression tests
// ---------------------------------------------------------------------------

type flaky struct{}

var flakyCalls int

func (f *flaky) Init() error {
	flakyCalls++
	if flakyCalls == 1 {
		return errors.New("transient")
	}
	return nil
}

func TestFailedInitCanBeRetried(t *testing.T) {
	flakyCalls = 0
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[flaky]()))

	assert.ErrorContains(t, c.Start(context.Background()), "transient")
	require.NoError(t, c.Start(context.Background()), "retry must not be reported as circular dependency")
}

type depObj struct{}
type ctorObj struct{ d *depObj }

func TestConstructorMissingDependencyReturnsError(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[ctorObj](),
		withConstructor(func(d *depObj) *ctorObj { return &ctorObj{d} })))

	assert.NotPanics(t, func() {
		assert.Error(t, c.Start(context.Background()))
	})
}

type notErr struct{}

func (notErr) Error() string { return "" }

func TestConstructorSecondReturnMustBeError(t *testing.T) {
	c := NewContainerImpl()
	err := c.Register(typeOf[ctorObj](),
		withConstructor(func() (*ctorObj, notErr) { return nil, notErr{} }))
	assert.Error(t, err)
}

type leafA struct{}
type leafB struct{}
type level3 struct {
	A *leafA `inject:""`
	B *leafB `inject:""`
}
type level2 struct{ L level3 }
type level1 struct{ L level2 }
type deepRoot struct{ L level1 }

func TestDeeplyNestedInjectFields(t *testing.T) {
	c := newStartedContainer(t, nil, typeOf[leafA](), typeOf[leafB](), typeOf[deepRoot]())

	obj, err := c.GetObject(typeOf[deepRoot]())
	require.NoError(t, err)
	r := obj.(*deepRoot)
	assert.NotNil(t, r.L.L.L.A)
	assert.NotNil(t, r.L.L.L.B)

	deps := c.objectManager.AllObjects()[2].Dependencies()
	require.Len(t, deps, 2)
	assert.Equal(t, "field L.L.L.A", deps[0].Location())
	assert.Equal(t, "field L.L.L.B", deps[1].Location())
}

type level int

type valueCfg struct {
	Level   level          `value:"lvl"`
	Timeout time.Duration  `value:"timeout"`
	Labels  map[string]int `value:"labels"`
	Names   []string       `value:"names"`
	Nested  nestedCfg      `value:"nested"`
	Ptr     *nestedCfg     `value:"nested"`
	Any     any            `value:"lvl"`
}

type nestedCfg struct {
	Opt  string `property:"opt;optional"`
	Port uint16 `property:"port"`
}

func TestValueConversion(t *testing.T) {
	c := newStartedContainer(t, mapProvider{
		"lvl":     3,
		"timeout": "5s",
		"labels":  map[string]any{"a": 1, "b": "2"},
		"names":   []string{"x", "y"},
		"nested":  map[string]any{"opt": "o", "port": 8080},
	}, typeOf[valueCfg]())

	obj, err := c.GetObject(typeOf[valueCfg]())
	require.NoError(t, err)
	cfg := obj.(*valueCfg)
	assert.Equal(t, level(3), cfg.Level)
	assert.Equal(t, 5*time.Second, cfg.Timeout)
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, cfg.Labels)
	assert.Equal(t, []string{"x", "y"}, cfg.Names)
	assert.Equal(t, nestedCfg{Opt: "o", Port: 8080}, cfg.Nested)
	assert.Equal(t, &nestedCfg{Opt: "o", Port: 8080}, cfg.Ptr)
	assert.Equal(t, 3, cfg.Any)
}

func TestValueConversionErrors(t *testing.T) {
	c := newStartedContainer(t, mapProvider{"s": "str", "f": 1.5, "big": 300})

	cases := []struct {
		key string
		rtp reflect.Type
	}{
		{"s", typeOf[[]string]()},
		{"s", typeOf[map[string]string]()},
		{"s", typeOf[nestedCfg]()},
		{"f", typeOf[int]()},
		{"big", typeOf[uint8]()},
		{"s", typeOf[chan int]()},
	}
	for _, tc := range cases {
		v, ok, err := c.GetValue(tc.key, tc.rtp)
		assert.Error(t, err, "%s -> %s", tc.key, tc.rtp)
		assert.False(t, ok)
		assert.Nil(t, v)
	}
}

type fooImpl struct{}

func (fooImpl) Foo() {}

type needsBar struct {
	B interface{ Bar() } `inject:";optional"`
}
type needsFoo struct {
	F interface{ Foo() } `inject:""`
}

func TestAnonymousInterfacesDoNotCollide(t *testing.T) {
	c := newStartedContainer(t, nil, typeOf[fooImpl](), typeOf[needsFoo](), typeOf[needsBar]())

	obj, err := c.GetObject(typeOf[needsBar]())
	require.NoError(t, err)
	assert.Nil(t, obj.(*needsBar).B)
}

func TestLocalTypesWithSameName(t *testing.T) {
	c := NewContainerImpl()
	{
		type local struct{}
		require.NoError(t, c.Register(typeOf[local]()))
	}
	{
		type local struct{}
		require.NoError(t, c.Register(typeOf[local]()), "distinct types must not be duplicates")
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

type concurrentObj struct {
	dep *depObj `inject:""`
}

func TestConcurrentGet(t *testing.T) {
	c := newStartedContainer(t, mapProvider{"k": "v"}, typeOf[depObj](), typeOf[concurrentObj]())

	var wg sync.WaitGroup
	results := make([]any, 50)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			obj, err := c.GetObject(typeOf[concurrentObj]())
			assert.NoError(t, err)
			_, _, err = c.GetValue("k", typeOf[string]())
			assert.NoError(t, err)
			_, err = c.GetObjectList(typeOf[depObj]())
			assert.NoError(t, err)
			results[i] = obj
		}(i)
	}
	wg.Wait()

	for _, r := range results[1:] {
		assert.Same(t, results[0], r, "singleton must be created once")
	}
}

func TestConcurrentStart(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[depObj]()))

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = c.Start(context.Background())
		}(i)
	}
	wg.Wait()

	var succeeded int
	for _, err := range errs {
		if err == nil {
			succeeded++
		} else {
			// depending on timing, the others see the container starting or started
			assert.True(t, errors.Is(err, ErrStarting) || errors.Is(err, ErrAlreadyStarted), "unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, succeeded)
}

func TestShutdownWhileGetting(t *testing.T) {
	c := newStartedContainer(t, nil, typeOf[depObj]())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.GetObject(typeOf[depObj]())
			if err != nil {
				assert.ErrorIs(t, err, ErrShutdown)
			}
		}()
	}
	require.NoError(t, c.Shutdown(context.Background()))
	wg.Wait()
}
