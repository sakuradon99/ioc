package ioc

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeOf[T any]() reflect.Type { return reflect.TypeOf(new(T)).Elem() }

func withName(name string) RegisterOption { return func(o *RegisterOptions) { o.Name = name } }
func withOptional() RegisterOption        { return func(o *RegisterOptions) { o.Optional = true } }
func withConstructor(c any) RegisterOption {
	return func(o *RegisterOptions) { o.Constructor = c }
}

type mapProvider map[string]any

func (m mapProvider) Provide() (map[string]any, error) { return m, nil }

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

	_, err := c.GetObject("", typeOf[flaky]())
	assert.EqualError(t, err, "transient")

	obj, err := c.GetObject("", typeOf[flaky]())
	require.NoError(t, err, "retry must not be reported as circular dependency")
	assert.NotNil(t, obj)
}

type optObj struct{}

func TestGetOptionalObjectDirectly(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[optObj](), withOptional()))

	obj, err := c.GetObject("", typeOf[optObj]())
	require.NoError(t, err)
	assert.IsType(t, &optObj{}, obj)

	list, err := c.GetObjectList("", typeOf[optObj]())
	require.NoError(t, err)
	assert.NotNil(t, list[0])
}

type depObj struct{}
type ctorObj struct{ d *depObj }

func TestConstructorMissingDependencyReturnsError(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[ctorObj](),
		withConstructor(func(d *depObj) *ctorObj { return &ctorObj{d} })))

	assert.NotPanics(t, func() {
		_, err := c.GetObject("", typeOf[ctorObj]())
		assert.Error(t, err)
	})
}

func TestConstructorInjectsNamedDependency(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[depObj](), withName("dep")))
	require.NoError(t, c.Register(typeOf[ctorObj](),
		withConstructor(func(d *depObj) *ctorObj { return &ctorObj{d} })))

	obj, err := c.GetObject("", typeOf[ctorObj]())
	require.NoError(t, err)
	assert.NotNil(t, obj.(*ctorObj).d)
}

func TestConstructorPrefersUnnamedDependency(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[depObj](), withName("other")))
	require.NoError(t, c.Register(typeOf[depObj]()))
	require.NoError(t, c.Register(typeOf[ctorObj](),
		withConstructor(func(d *depObj) *ctorObj { return &ctorObj{d} })))

	_, err := c.GetObject("", typeOf[ctorObj]())
	require.NoError(t, err)
}

func TestConstructorReturnsNil(t *testing.T) {
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[ctorObj](),
		withConstructor(func() (*ctorObj, error) { return nil, nil })))

	_, err := c.GetObject("", typeOf[ctorObj]())
	assert.Error(t, err)
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
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[leafA]()))
	require.NoError(t, c.Register(typeOf[leafB]()))
	require.NoError(t, c.Register(typeOf[deepRoot]()))

	obj, err := c.GetObject("", typeOf[deepRoot]())
	require.NoError(t, err)
	r := obj.(*deepRoot)
	assert.NotNil(t, r.L.L.L.A)
	assert.NotNil(t, r.L.L.L.B)
}

type reentrant struct{ peer *leafA }

var reentrantContainer *ContainerImpl

func (r *reentrant) Init() error {
	if _, _, err := reentrantContainer.GetValue("k", typeOf[string]()); err != nil {
		return err
	}
	obj, err := reentrantContainer.GetObject("", typeOf[leafA]())
	if err != nil {
		return err
	}
	r.peer = obj.(*leafA)
	return nil
}

func TestInitCanCallContainer(t *testing.T) {
	reentrantContainer = NewContainerImpl()
	reentrantContainer.AddValueProvider(mapProvider{"k": "v"})
	require.NoError(t, reentrantContainer.Register(typeOf[reentrant]()))
	require.NoError(t, reentrantContainer.Register(typeOf[leafA]()))

	done := make(chan error, 1)
	go func() {
		_, err := reentrantContainer.GetObject("", typeOf[reentrant]())
		done <- err
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("deadlock: Init() calling the container blocked")
	}
}

func TestLateValueProvider(t *testing.T) {
	c := NewContainerImpl()
	c.AddValueProvider(mapProvider{"a": "1"})
	_, _, err := c.GetValue("a", typeOf[string]())
	require.NoError(t, err)

	c.AddValueProvider(mapProvider{"a": "override", "b": "2"})
	v, ok, err := c.GetValue("b", typeOf[string]())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "2", v)

	v, _, _ = c.GetValue("a", typeOf[string]())
	assert.Equal(t, "override", v, "later provider has higher priority")
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
	c := NewContainerImpl()
	c.AddValueProvider(mapProvider{
		"lvl":     3,
		"timeout": "5s",
		"labels":  map[string]any{"a": 1, "b": "2"},
		"names":   []string{"x", "y"},
		"nested":  map[string]any{"opt": "o", "port": 8080},
	})
	require.NoError(t, c.Register(typeOf[valueCfg]()))

	obj, err := c.GetObject("", typeOf[valueCfg]())
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
	c := NewContainerImpl()
	c.AddValueProvider(mapProvider{"s": "str", "f": 1.5, "big": 300, "fn": func() {}})

	cases := []struct {
		key string
		rtp reflect.Type
	}{
		{"s", typeOf[[]string]()},
		{"s", typeOf[map[string]string]()},
		{"s", typeOf[nestedCfg]()},
		{"f", typeOf[int]()},
		{"big", typeOf[uint8]()},
		{"fn", typeOf[chan int]()},
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
	c := NewContainerImpl()
	require.NoError(t, c.Register(typeOf[fooImpl]()))
	require.NoError(t, c.Register(typeOf[needsFoo]()))
	require.NoError(t, c.Register(typeOf[needsBar]()))

	obj, err := c.GetObject("", typeOf[needsBar]())
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
