package ioc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// lifecycleLog records Init and Destroy calls in order.
var lifecycleLog []string

type ctxKey struct{}

// ctxLifecycle implements the context variants of Init and Destroy.
type ctxLifecycle struct {
	dep       *plainLifecycle `inject:""`
	initValue any
}

func (o *ctxLifecycle) Init(ctx context.Context) error {
	o.initValue = ctx.Value(ctxKey{})
	lifecycleLog = append(lifecycleLog, "init ctx")
	return nil
}

func (o *ctxLifecycle) Destroy(ctx context.Context) error {
	lifecycleLog = append(lifecycleLog, "destroy ctx")
	return nil
}

// plainLifecycle implements the variants of Init and Destroy without context.
type plainLifecycle struct{}

func (o *plainLifecycle) Init() error {
	lifecycleLog = append(lifecycleLog, "init plain")
	return nil
}

func (o *plainLifecycle) Destroy() error {
	lifecycleLog = append(lifecycleLog, "destroy plain")
	return errors.New("destroy plain failed")
}

type failingDestroy struct{}

func (o *failingDestroy) Destroy() error { return errors.New("destroy failed") }

// callsContainerInInit calls the container from Init, which is not allowed.
type callsContainerInInit struct{}

var errsFromInit []error

func (o *callsContainerInInit) Init() error {
	_, err := GetObject[valueHolder]("")
	errsFromInit = append(errsFromInit, err)
	_, _, err = GetValue[string]("any")
	errsFromInit = append(errsFromInit, err)
	errsFromInit = append(errsFromInit, iocContainer.Register(reflect.TypeOf(englishGreeter{})))
	errsFromInit = append(errsFromInit, AddValueProvider(NewMapValueProvider(nil)))
	errsFromInit = append(errsFromInit, Start(context.Background()))
	errsFromInit = append(errsFromInit, Shutdown(context.Background()))
	return nil
}

type namedPlugin struct {
	plugin Plugin `inject:"plugin1"`
}

// greetFunc is a non-struct type implementing Greeter.
type greetFunc func() string

func (f greetFunc) Greet() string { return f() }

type greeterUser struct {
	greeter Greeter `inject:""`
}

// providedLifecycle records whether the container called its hooks.
type providedLifecycle struct {
	initCalled    bool
	destroyCalled bool
	injected      *englishGreeter `inject:""`
}

func (o *providedLifecycle) Init() error {
	o.initCalled = true
	return nil
}

func (o *providedLifecycle) Destroy() error {
	o.destroyCalled = true
	return nil
}

type dbConfig struct {
	host string `property:"host;default=127.0.0.1"`
	port int    `property:"port"`
}

type defaultValues struct {
	port    int           `value:"server.port;default=8080"`
	host    string        `value:"server.host;default=localhost"`
	timeout time.Duration `value:"timeout;default=5s"`
	retries *int          `value:"retries;default=3"`
	level   logLevel      `value:"level;default=2"`
	empty   string        `value:"empty;default="`
	db      dbConfig      `value:"db"`
}

type invalidDefault struct {
	n int `value:"n;default=abc"`
}

type unsupportedDefault struct {
	list []string `value:"list;default=a"`
}

type invalidPropertyDefault struct {
	cfg struct {
		n int `property:"n;default=x"`
	} `value:"cfg"`
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestStartIsRequired(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder]()

	_, err := GetObject[valueHolder]("")
	assert.ErrorIs(t, err, ErrNotStarted)
	_, err = GetObjectList[valueHolder]("")
	assert.ErrorIs(t, err, ErrNotStarted)
	_, err = GetInterfaceMap[Plugin]("")
	assert.ErrorIs(t, err, ErrNotStarted)
	_, _, err = GetValue[string]("str")
	assert.ErrorIs(t, err, ErrNotStarted)

	startContainer(t)

	_, err = GetObject[valueHolder]("")
	assert.NoError(t, err)
	_, _, err = GetValue[string]("str")
	assert.NoError(t, err)
}

func TestContainerIsReadOnlyAfterStart(t *testing.T) {
	newTestContainer(t)
	startContainer(t)

	assert.PanicsWithError(t, ErrAlreadyStarted.Error(), func() { Register[englishGreeter]() })
	assert.PanicsWithError(t, ErrAlreadyStarted.Error(), func() { Provide(&englishGreeter{}) })
	assert.ErrorIs(t, AddValueProvider(NewMapValueProvider(nil)), ErrAlreadyStarted)
	assert.ErrorIs(t, Start(context.Background()), ErrAlreadyStarted)
}

func TestContainerCannotBeCalledFromInit(t *testing.T) {
	newTestContainer(t)
	errsFromInit = nil
	Register[callsContainerInInit]()

	require.NoError(t, Start(context.Background()), "must not deadlock")
	require.Len(t, errsFromInit, 6)
	for i, err := range errsFromInit {
		assert.ErrorIs(t, err, ErrStarting, "call #%d", i)
	}
}

func TestStartCanBeRetriedAfterFailure(t *testing.T) {
	newTestContainer(t)
	Register[requiredDependency]()

	assert.ErrorContains(t, Start(context.Background()), "missing implementation")

	_, err := GetObject[requiredDependency]("")
	assert.ErrorIs(t, err, ErrNotStarted, "a failed Start leaves the container not started")

	Register[englishGreeter]()
	startContainer(t)

	obj, err := GetObject[requiredDependency]("")
	require.NoError(t, err)
	assert.Equal(t, "hello", obj.greeter.Greet())
}

func TestStartWithCanceledContext(t *testing.T) {
	newTestContainer(t)
	Register[englishGreeter]()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, Start(ctx), context.Canceled)
}

func TestInitAndDestroyHooks(t *testing.T) {
	newTestContainer(t)
	lifecycleLog = nil
	Register[ctxLifecycle]()
	Register[plainLifecycle]()
	Register[failingDestroy]()

	ctx := context.WithValue(context.Background(), ctxKey{}, "from start")
	require.NoError(t, Start(ctx))

	obj, err := GetObject[ctxLifecycle]("")
	require.NoError(t, err)
	assert.Equal(t, "from start", obj.initValue, "Init receives the context passed to Start")
	assert.Equal(t, []string{"init plain", "init ctx"}, lifecycleLog, "dependencies are initialized first")

	lifecycleLog = nil
	err = Shutdown(context.Background())
	assert.Equal(t, []string{"destroy ctx", "destroy plain"}, lifecycleLog, "destroyed in reverse initialization order")
	assert.ErrorContains(t, err, `destroy <github.com/sakuradon99/ioc.failingDestroy@"">: destroy failed`)
	assert.ErrorContains(t, err, `destroy <github.com/sakuradon99/ioc.plainLifecycle@"">: destroy plain failed`)

	assert.NoError(t, Shutdown(context.Background()), "Shutdown is idempotent")
	_, err = GetObject[ctxLifecycle]("")
	assert.ErrorIs(t, err, ErrShutdown)
	assert.ErrorIs(t, Start(context.Background()), ErrShutdown)
}

func TestShutdownWithExpiredContext(t *testing.T) {
	newTestContainer(t)
	lifecycleLog = nil
	Register[plainLifecycle]()
	startContainer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, Shutdown(ctx), context.Canceled)
	assert.Equal(t, []string{"init plain"}, lifecycleLog, "no Destroy is called after the context is done")
}

func TestShutdownAfterFailedStart(t *testing.T) {
	newTestContainer(t)
	lifecycleLog = nil
	Register[plainLifecycle]()
	Register[requiredDependency]()

	require.Error(t, Start(context.Background()))
	assert.ErrorContains(t, Shutdown(context.Background()), "destroy plain failed",
		"objects initialized before the failure are destroyed")
	assert.Equal(t, []string{"init plain", "destroy plain"}, lifecycleLog)
}

// ---------------------------------------------------------------------------
// Matching by empty name and Primary
// ---------------------------------------------------------------------------

func TestEmptyNameMatching(t *testing.T) {
	tests := []struct {
		name     string
		register func()
		want     string
		wantErr  string
	}{
		{
			name:     "the only named object",
			register: func() { Register[pluginOne](Name("plugin1")) },
			want:     "plugin1",
		},
		{
			name: "unnamed object is preferred",
			register: func() {
				Register[pluginOne]()
				Register[pluginTwo](Name("plugin2"))
			},
			want: "plugin1",
		},
		{
			name: "primary object among named ones",
			register: func() {
				Register[pluginOne](Name("plugin1"))
				Register[pluginTwo](Name("plugin2"), Primary())
			},
			want: "plugin2",
		},
		{
			name: "unnamed object is preferred over primary",
			register: func() {
				Register[pluginOne]()
				Register[pluginTwo](Name("plugin2"), Primary())
			},
			want: "plugin1",
		},
		{
			name: "multiple primary objects",
			register: func() {
				Register[pluginOne](Name("plugin1"), Primary())
				Register[pluginTwo](Name("plugin2"), Primary())
			},
			wantErr: "multiple implementations found",
		},
		{
			name: "multiple named objects without primary",
			register: func() {
				Register[pluginOne](Name("plugin1"))
				Register[pluginTwo](Name("plugin2"))
			},
			wantErr: "multiple implementations found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTestContainer(t)
			tt.register()
			Register[singlePlugin]()

			err := Start(context.Background())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			obj, err := GetObject[singlePlugin]("")
			require.NoError(t, err)
			assert.Equal(t, tt.want, obj.plugin.ID(), "inject")

			p, err := GetInterface[Plugin]("")
			require.NoError(t, err)
			assert.Equal(t, tt.want, p.ID(), "GetInterface")
		})
	}
}

func TestPrimaryIsIgnoredWithName(t *testing.T) {
	newTestContainer(t)
	Register[pluginOne](Name("plugin1"))
	Register[pluginTwo](Name("plugin2"), Primary())
	Register[namedPlugin]()
	Register[optionalPluginList]()
	startContainer(t)

	obj, err := GetObject[namedPlugin]("")
	require.NoError(t, err)
	assert.Equal(t, "plugin1", obj.plugin.ID())

	list, err := GetObject[optionalPluginList]("")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"plugin1", "plugin2"}, pluginIDs(list.plugins), "empty name in list matches all")
}

// ---------------------------------------------------------------------------
// Provide
// ---------------------------------------------------------------------------

func TestProvide(t *testing.T) {
	t.Run("pointer to struct", func(t *testing.T) {
		newTestContainer(t)
		instance := &englishGreeter{}
		Provide(instance)
		Register[greeterUser]()
		startContainer(t)

		obj, err := GetObject[englishGreeter]("")
		require.NoError(t, err)
		assert.Same(t, instance, obj)

		user, err := GetObject[greeterUser]("")
		require.NoError(t, err)
		assert.Same(t, instance, user.greeter)
	})

	t.Run("instance passed as interface", func(t *testing.T) {
		newTestContainer(t)
		var greeter Greeter = &englishGreeter{}
		Provide(greeter, Name("greeter"))
		startContainer(t)

		obj, err := GetObject[englishGreeter]("greeter")
		require.NoError(t, err, "registered by its dynamic type")
		assert.Same(t, greeter, obj)
	})

	t.Run("non struct type", func(t *testing.T) {
		newTestContainer(t)
		Provide(greetFunc(func() string { return "hi" }))
		Register[greeterUser]()
		startContainer(t)

		user, err := GetObject[greeterUser]("")
		require.NoError(t, err)
		assert.Equal(t, "hi", user.greeter.Greet())
	})

	t.Run("lifecycle hooks and injection are skipped", func(t *testing.T) {
		newTestContainer(t)
		instance := &providedLifecycle{}
		Provide(instance)
		Register[englishGreeter]()
		require.NoError(t, Start(context.Background()))
		require.NoError(t, Shutdown(context.Background()))

		assert.False(t, instance.initCalled)
		assert.False(t, instance.destroyCalled)
		assert.Nil(t, instance.injected)
	})

	t.Run("invalid instances", func(t *testing.T) {
		newTestContainer(t)
		var nilPtr *englishGreeter
		var nilFunc greetFunc
		tests := map[string]func(){
			"nil":                func() { Provide(nil) },
			"nil pointer":        func() { Provide(nilPtr) },
			"nil func":           func() { Provide(nilFunc) },
			"struct value":       func() { Provide(englishGreeter{}) },
			"constructor option": func() { Provide(&englishGreeter{}, Constructor(func() *englishGreeter { return nil })) },
		}
		for name, provide := range tests {
			assert.Panics(t, provide, name)
		}
	})

	t.Run("duplicate with registered object", func(t *testing.T) {
		newTestContainer(t)
		Register[englishGreeter]()
		assert.PanicsWithError(t, `object <github.com/sakuradon99/ioc.englishGreeter@""> is already registered`, func() {
			Provide(&englishGreeter{})
		})
	})
}

// ---------------------------------------------------------------------------
// Default values
// ---------------------------------------------------------------------------

func TestDefaultValues(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"server": map[string]any{"host": "example.com"},
		"db":     map[string]any{"port": 5432},
	})))
	Register[defaultValues]()
	startContainer(t)

	obj, err := GetObject[defaultValues]("")
	require.NoError(t, err)

	assert.Equal(t, 8080, obj.port, "default is used when missing")
	assert.Equal(t, "example.com", obj.host, "provided value wins over default")
	assert.Equal(t, 5*time.Second, obj.timeout)
	require.NotNil(t, obj.retries)
	assert.Equal(t, 3, *obj.retries)
	assert.Equal(t, logLevel(2), obj.level)
	assert.Empty(t, obj.empty)
	assert.Equal(t, dbConfig{host: "127.0.0.1", port: 5432}, obj.db, "property default")
}

func TestInvalidDefaultValues(t *testing.T) {
	tests := map[string]func(){
		"not convertible":          func() { Register[invalidDefault]() },
		"unsupported type":         func() { Register[unsupportedDefault]() },
		"invalid property default": func() { Register[invalidPropertyDefault]() },
	}
	for name, register := range tests {
		t.Run(name, func(t *testing.T) {
			newTestContainer(t)
			assert.Panics(t, register, "invalid defaults are reported on Register")
		})
	}
}

func TestPresentValueNotConvertibleIsNotDefaulted(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"server": map[string]any{"port": "not a number"},
	})))
	Register[defaultValues]()

	assert.ErrorContains(t, Start(context.Background()), `field port: value "server.port": cannot convert`)
}

// ---------------------------------------------------------------------------
// Dump
// ---------------------------------------------------------------------------

func TestDump(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"key1": "a", "key2": "b", "next": map[string]any{"key1": "c"},
	})))
	Register[englishGreeter](Alias("greeterAlias"), Primary())
	Register[aliasConsumer]()
	Register[mapValueTarget]()
	Register[pluginOne](Conditional("#key1 == 'x'"))
	Register[pluginTwo](Optional())
	Provide(greetFunc(func() string { return "" }), Name("func"))

	before := Dump()
	assert.Contains(t, before, "container: new, 6 object(s)")
	assert.Contains(t, before, `condition="#key1 == 'x'"]`)

	startContainer(t)
	dump := Dump()
	t.Log("\n" + dump)

	for _, want := range []string{
		"container: started, 6 object(s)",
		`* github.com/sakuradon99/ioc.englishGreeter@"" aliases=[greeterAlias] [initialized, primary]`,
		`    field greeter <- github.com/sakuradon99/ioc.Greeter@"greeterAlias"`,
		`    field first <- value "key1"`,
		`    field nestedFirst <- value "next.key1"`,
		`[not initialized, condition="#key1 == 'x'" -> false]`,
		`* github.com/sakuradon99/ioc.pluginTwo@"" [not initialized, optional]`,
		`* github.com/sakuradon99/ioc.greetFunc@"func" [initialized, provided]`,
	} {
		assert.Contains(t, dump, want)
	}
}
