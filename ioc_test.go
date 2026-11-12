package ioc

import (
	"errors"
	"testing"
	"time"

	ioc "github.com/sakuradon99/ioc/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfigFile = "testdata/config.yaml"

// newTestContainer replaces the global container with a fresh one for the duration of the test.
func newTestContainer(t *testing.T) {
	t.Helper()
	previous := iocContainer
	iocContainer = ioc.NewContainerImpl()
	t.Cleanup(func() { iocContainer = previous })
}

// newTestContainerWithConfig creates a fresh container and loads testdata/config.yaml into it.
func newTestContainerWithConfig(t *testing.T) {
	t.Helper()
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewFileValueProvider(testConfigFile)))
}

// ---------------------------------------------------------------------------
// Fixtures: values
// ---------------------------------------------------------------------------

// basicProps maps every supported basic kind from a nested config section via `property` tags.
type basicProps struct {
	text        string    `property:"str"`
	textPtr     *string   `property:"str_ptr"`
	missingText *string   `property:"str_opt;optional"`
	count       int       `property:"int"`
	ratio       float32   `property:"float"`
	enabled     bool      `property:"bool"`
	textList    []string  `property:"string_arr"`
	textPtrList []*string `property:"string_ptr_arr"`
}

// embeddedProps reads the properties of an untagged embedded struct from the same config level.
type embeddedProps struct {
	basicProps
	extraText string `property:"str_2"`
}

// valueHolder receives config values through `value` tags.
type valueHolder struct {
	text        string        `value:"str"`
	textPtr     *string       `value:"str_ptr"`
	missingText *string       `value:"str_opt;optional"`
	count       int           `value:"int"`
	ratio       float32       `value:"float"`
	enabled     bool          `value:"bool"`
	textList    []string      `value:"string_arr"`
	textPtrList []*string     `value:"string_ptr_arr"`
	nestedText  string        `value:"next.str"`
	nestedCount int           `value:"next.int"`
	props       basicProps    `value:"property_stu"`
	embedded    embeddedProps `value:"nested_stu"`
}

type logLevel int

// typedValues covers named types, durations, maps and pointer-to-struct values.
type typedValues struct {
	level    logLevel          `value:"level"`
	timeout  time.Duration     `value:"timeout"`
	weights  map[string]int    `value:"weights"`
	settings *portSettings     `value:"settings"`
	anything any               `value:"level"`
	labels   map[string]string `value:"labels;optional"`
}

type portSettings struct {
	port uint16 `property:"port"`
}

// ---------------------------------------------------------------------------
// Fixtures: objects
// ---------------------------------------------------------------------------

type holderRef struct {
	holder *valueHolder `inject:""`
}

type embedsHolder struct {
	valueHolder
}

type embedsHolderRef struct {
	embedded *embedsHolder `inject:""`
}

type anotherHolderRef struct {
	holder *valueHolder `inject:""`
}

// nestedInjection injects through a direct field, an embedded struct and an anonymous struct field.
type nestedInjection struct {
	ref *holderRef `inject:""`
	embedsHolderRef
	group struct {
		other *anotherHolderRef `inject:""`
	}
}

// ---------------------------------------------------------------------------
// Fixtures: interfaces
// ---------------------------------------------------------------------------

type Greeter interface {
	Greet() string
}

type englishGreeter struct{}

func (g *englishGreeter) Greet() string { return "hello" }

type Plugin interface {
	ID() string
}

type pluginOne struct{}

func (p *pluginOne) ID() string { return "plugin1" }

type pluginTwo struct{}

func (p *pluginTwo) ID() string { return "plugin2" }

type interfaceConsumer struct {
	holder      *valueHolder
	optionalRef *holderRef `inject:";optional"`
	greeter     Greeter    `inject:""`
	first       Plugin     `inject:"plugin1"`
	second      Plugin     `inject:"plugin2"`

	text     string
	untagged string
}

type consumerParams struct {
	first    Plugin `inject:"plugin1"`
	second   Plugin `inject:"plugin2"`
	untagged string
	text     string `value:"str"`
}

func newInterfaceConsumer(holder *valueHolder, greeter Greeter, params consumerParams) *interfaceConsumer {
	return &interfaceConsumer{
		holder:   holder,
		greeter:  greeter,
		first:    params.first,
		second:   params.second,
		text:     params.text,
		untagged: params.untagged,
	}
}

type singlePlugin struct {
	plugin Plugin `inject:""`
}

type optionalPluginList struct {
	plugins []Plugin `inject:";optional"`
}

type holderList struct {
	holders []*valueHolder `inject:"holder[12]"`
}

type filteredPluginList struct {
	plugins []Plugin `inject:"*2"`
}

type pluginMap struct {
	plugins map[string]Plugin `inject:"*"`
}

type aliasConsumer struct {
	greeter Greeter `inject:"greeterAlias"`
}

type optionalDependency struct {
	greeter Greeter      `inject:";optional"`
	holder  *valueHolder `inject:";optional"`
}

type requiredDependency struct {
	greeter Greeter `inject:""`
}

type requiredObjectDependency struct {
	holder *valueHolder `inject:""`
}

type initHook struct {
	holder *valueHolder `inject:""`
}

func (o *initHook) Init() error {
	o.holder.text = "init"
	return nil
}

type failingInit struct{}

func (f *failingInit) Init() error { return errors.New("init failed") }

type mapValueTarget struct {
	first       string `value:"key1"`
	second      string `value:"key2"`
	nestedFirst string `value:"next.key1"`
}

type circularA struct {
	b *circularB `inject:""`
}

type circularB struct {
	a *circularA `inject:""`
}

type unsupportedInjectField struct {
	count int `inject:""`
}

// ---------------------------------------------------------------------------
// Value injection
// ---------------------------------------------------------------------------

func TestValueInjection(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder]()

	holder, err := GetObject[valueHolder]("")
	require.NoError(t, err)

	assert.Equal(t, "str", holder.text)
	assert.Equal(t, "str_ptr", *holder.textPtr)
	assert.Nil(t, holder.missingText)
	assert.Equal(t, 1, holder.count)
	assert.Equal(t, float32(0.99), holder.ratio)
	assert.True(t, holder.enabled)
	assert.Equal(t, []string{"str_1", "str_2"}, holder.textList)
	assert.Equal(t, []string{"str_ptr_1", "str_ptr_2"}, derefAll(holder.textPtrList))
	assert.Equal(t, "next_str", holder.nestedText)
	assert.Equal(t, 2, holder.nestedCount)

	t.Run("struct value via property tags", func(t *testing.T) {
		assertBasicProps(t, holder.props)
	})

	t.Run("embedded struct value", func(t *testing.T) {
		assertBasicProps(t, holder.embedded.basicProps)
		assert.Equal(t, "str_2", holder.embedded.extraText)
	})
}

func TestTypedValueInjection(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"level":    3,
		"timeout":  "1m30s",
		"weights":  map[string]any{"a": 1, "b": "2"},
		"settings": map[string]any{"port": 8080},
	})))
	Register[typedValues]()

	values, err := GetObject[typedValues]("")
	require.NoError(t, err)

	assert.Equal(t, logLevel(3), values.level)
	assert.Equal(t, 90*time.Second, values.timeout)
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, values.weights)
	assert.Equal(t, &portSettings{port: 8080}, values.settings)
	assert.Equal(t, 3, values.anything)
	assert.Nil(t, values.labels)
}

func TestMapValueProvider(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"key1": "value1",
		"key2": "value2",
		"next": map[string]any{"key1": "next_value1"},
	})))
	Register[mapValueTarget]()

	target, err := GetObject[mapValueTarget]("")
	require.NoError(t, err)

	assert.Equal(t, "value1", target.first)
	assert.Equal(t, "value2", target.second)
	assert.Equal(t, "next_value1", target.nestedFirst)
}

func TestFileValueProvider(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		key     string
		want    string
		wantErr string
	}{
		{name: "yaml", file: testConfigFile, key: "next.str", want: "next_str"},
		{name: "json", file: "testdata/config.json", key: "app.name", want: "json-app"},
		{name: "upper case extension", file: "testdata/CONFIG_UPPER.YAML", key: "app.name", want: "upper-app"},
		{name: "unsupported extension", file: "testdata/config.unsupported.toml", wantErr: "unsupported file format"},
		{name: "missing file", file: "testdata/not_exist.yaml", wantErr: "no such file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := NewFileValueProvider(tt.file).Provide()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			newTestContainer(t)
			require.NoError(t, AddValueProvider(NewMapValueProvider(values)))
			got, ok, err := GetValue[string](tt.key)
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValueProviderPriority(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{"name": "first", "only_first": "x"})))

	name, _, err := GetValue[string]("name")
	require.NoError(t, err)
	assert.Equal(t, "first", name)

	// providers added after the first read are still loaded, and the last added one wins
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{"name": "second"})))

	name, _, err = GetValue[string]("name")
	require.NoError(t, err)
	assert.Equal(t, "second", name)

	onlyFirst, ok, err := GetValue[string]("only_first")
	require.NoError(t, err)
	assert.True(t, ok, "keys missing in later providers fall back to earlier ones")
	assert.Equal(t, "x", onlyFirst)
}

func TestGetValue(t *testing.T) {
	newTestContainer(t)
	require.NoError(t, AddValueProvider(NewMapValueProvider(map[string]any{
		"str":     "abc",
		"num":     "42",
		"float":   1.5,
		"big":     300,
		"list":    []any{"a", "b"},
		"timeout": "5s",
		"section": map[string]any{"port": 80},
	})))

	t.Run("string", func(t *testing.T) {
		v, ok, err := GetValue[string]("str")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "abc", v)
	})

	t.Run("string converted to int", func(t *testing.T) {
		v, ok, err := GetValue[int]("num")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 42, v)
	})

	t.Run("slice", func(t *testing.T) {
		v, _, err := GetValue[[]string]("list")
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, v)
	})

	t.Run("duration", func(t *testing.T) {
		v, _, err := GetValue[time.Duration]("timeout")
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, v)
	})

	t.Run("map", func(t *testing.T) {
		v, _, err := GetValue[map[string]int]("section")
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"port": 80}, v)
	})

	t.Run("missing key", func(t *testing.T) {
		v, ok, err := GetValue[string]("not_exist")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, v)
	})

	errorCases := []struct {
		name string
		get  func() error
	}{
		{"string to int", func() error { _, _, err := GetValue[int]("str"); return err }},
		{"string to slice", func() error { _, _, err := GetValue[[]string]("str"); return err }},
		{"string to map", func() error { _, _, err := GetValue[map[string]string]("str"); return err }},
		{"string to struct", func() error { _, _, err := GetValue[portSettings]("str"); return err }},
		{"fraction to int", func() error { _, _, err := GetValue[int]("float"); return err }},
		{"overflow", func() error { _, _, err := GetValue[uint8]("big"); return err }},
		{"unsupported type", func() error { _, _, err := GetValue[chan int]("str"); return err }},
	}
	for _, tc := range errorCases {
		t.Run("error: "+tc.name, func(t *testing.T) {
			var err error
			assert.NotPanics(t, func() { err = tc.get() })
			assert.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Object injection
// ---------------------------------------------------------------------------

func TestObjectInjection(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder]()
	Register[holderRef]()
	Register[embedsHolder]()
	Register[embedsHolderRef]()
	Register[anotherHolderRef]()
	Register[nestedInjection]()

	obj, err := GetObject[nestedInjection]("")
	require.NoError(t, err)

	assert.Equal(t, "str", obj.ref.holder.text, "direct field")
	assert.Equal(t, "str", obj.embedded.text, "field of embedded struct")
	assert.Equal(t, "str", obj.group.other.holder.text, "field of anonymous struct")
	assert.Same(t, obj.ref.holder, obj.group.other.holder, "objects are singletons")
}

func TestInterfaceInjection(t *testing.T) {
	newTestContainer(t)
	Register[englishGreeter]()
	Register[pluginOne](Name("plugin1"))
	Register[pluginTwo](Name("plugin2"))
	Register[interfaceConsumer]()

	consumer, err := GetObject[interfaceConsumer]("")
	require.NoError(t, err)

	assert.Nil(t, consumer.optionalRef)
	assert.Nil(t, consumer.holder, "untagged field is not injected")
	assert.Equal(t, "hello", consumer.greeter.Greet())
	assert.Equal(t, "plugin1", consumer.first.ID())
	assert.Equal(t, "plugin2", consumer.second.ID())
}

func TestAliasInjection(t *testing.T) {
	newTestContainer(t)
	Register[englishGreeter](Alias("greeterAlias"))
	Register[aliasConsumer]()

	consumer, err := GetObject[aliasConsumer]("")
	require.NoError(t, err)
	require.NotNil(t, consumer.greeter)
	assert.Equal(t, "hello", consumer.greeter.Greet())
}

func TestOptionalInjection(t *testing.T) {
	newTestContainer(t)
	Register[optionalDependency]()

	obj, err := GetObject[optionalDependency]("")
	require.NoError(t, err)
	assert.Nil(t, obj.greeter)
	assert.Nil(t, obj.holder)
}

func TestConstructorInjection(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder](Optional())
	Register[holderRef]()
	Register[englishGreeter]()
	Register[pluginOne](Name("plugin1"))
	Register[pluginTwo](Name("plugin2"))
	Register[interfaceConsumer](Constructor(newInterfaceConsumer))

	consumer, err := GetObject[interfaceConsumer]("")
	require.NoError(t, err)

	assert.Equal(t, "str", consumer.holder.text)
	assert.Equal(t, "hello", consumer.greeter.Greet())
	assert.Equal(t, "plugin1", consumer.first.ID())
	assert.Equal(t, "plugin2", consumer.second.ID())
	assert.Equal(t, "str", consumer.text, "value tag in params struct")
	assert.Empty(t, consumer.untagged)
}

func TestConstructorDependencyResolution(t *testing.T) {
	newHolderRef := func(holder *valueHolder) *holderRef { return &holderRef{holder: holder} }

	t.Run("falls back to the only named object", func(t *testing.T) {
		newTestContainerWithConfig(t)
		Register[valueHolder](Name("main"))
		Register[holderRef](Constructor(newHolderRef))

		ref, err := GetObject[holderRef]("")
		require.NoError(t, err)
		assert.NotNil(t, ref.holder)
	})

	t.Run("prefers the unnamed object", func(t *testing.T) {
		newTestContainerWithConfig(t)
		Register[valueHolder](Name("other"))
		Register[valueHolder]()
		Register[holderRef](Constructor(newHolderRef))

		ref, err := GetObject[holderRef]("")
		require.NoError(t, err)

		unnamed, err := GetObject[valueHolder]("")
		require.NoError(t, err)
		assert.Same(t, unnamed, ref.holder)
	})

	t.Run("ambiguous named objects", func(t *testing.T) {
		newTestContainerWithConfig(t)
		Register[valueHolder](Name("a"))
		Register[valueHolder](Name("b"))
		Register[holderRef](Constructor(newHolderRef))

		_, err := GetObject[holderRef]("")
		assert.ErrorContains(t, err, "multiple objects found")
	})

	t.Run("missing dependency", func(t *testing.T) {
		newTestContainer(t)
		Register[holderRef](Constructor(newHolderRef))

		_, err := GetObject[holderRef]("")
		assert.ErrorContains(t, err, "missing object")
	})

	t.Run("constructor error", func(t *testing.T) {
		newTestContainer(t)
		Register[holderRef](Constructor(func() (*holderRef, error) { return nil, errors.New("boom") }))

		_, err := GetObject[holderRef]("")
		assert.EqualError(t, err, "boom")
	})

	t.Run("constructor returns nil", func(t *testing.T) {
		newTestContainer(t)
		Register[holderRef](Constructor(func() *holderRef { return nil }))

		_, err := GetObject[holderRef]("")
		assert.ErrorContains(t, err, "returned nil object")
	})
}

func TestConditionalRegistration(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[pluginOne](Conditional("#condition.use_impl_multi == 1"))
	Register[pluginTwo](Conditional("#condition.use_impl_multi == 2"))
	Register[singlePlugin]()

	obj, err := GetObject[singlePlugin]("")
	require.NoError(t, err)
	assert.Equal(t, "plugin2", obj.plugin.ID())
}

// ---------------------------------------------------------------------------
// Collection injection
// ---------------------------------------------------------------------------

func TestListInjection(t *testing.T) {
	t.Run("all implementations", func(t *testing.T) {
		newTestContainer(t)
		Register[pluginOne]()
		Register[pluginTwo]()
		Register[optionalPluginList]()

		obj, err := GetObject[optionalPluginList]("")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"plugin1", "plugin2"}, pluginIDs(obj.plugins))
	})

	t.Run("optional list without implementations", func(t *testing.T) {
		newTestContainer(t)
		Register[optionalPluginList]()

		obj, err := GetObject[optionalPluginList]("")
		require.NoError(t, err)
		assert.Empty(t, obj.plugins)
	})

	t.Run("struct pointers filtered by name", func(t *testing.T) {
		newTestContainerWithConfig(t)
		Register[valueHolder](Name("holder1"))
		Register[valueHolder](Name("holder2"))
		Register[valueHolder](Name("holder3"))
		Register[holderList]()

		obj, err := GetObject[holderList]("")
		require.NoError(t, err)
		require.Len(t, obj.holders, 2)
		assert.Equal(t, "str", obj.holders[0].text)
		assert.Equal(t, "str", obj.holders[1].text)
	})

	t.Run("interfaces filtered by name", func(t *testing.T) {
		newTestContainer(t)
		Register[pluginOne](Name("plugin1"))
		Register[pluginTwo](Name("plugin2"))
		Register[filteredPluginList]()

		obj, err := GetObject[filteredPluginList]("")
		require.NoError(t, err)
		assert.Equal(t, []string{"plugin2"}, pluginIDs(obj.plugins))
	})
}

func TestMapInjection(t *testing.T) {
	newTestContainer(t)
	Register[pluginOne](Name("plugin1"))
	Register[pluginTwo](Name("plugin2"))
	Register[pluginMap]()

	obj, err := GetObject[pluginMap]("")
	require.NoError(t, err)
	require.Len(t, obj.plugins, 2)
	assert.Equal(t, "plugin1", obj.plugins["plugin1"].ID())
	assert.Equal(t, "plugin2", obj.plugins["plugin2"].ID())
}

// ---------------------------------------------------------------------------
// Get APIs
// ---------------------------------------------------------------------------

func TestGetObjectAPIs(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder](Name("holder1"))
	Register[valueHolder](Name("holder2"))
	Register[valueHolder](Name("holder3"))

	t.Run("object by name", func(t *testing.T) {
		obj, err := GetObject[valueHolder]("holder2")
		require.NoError(t, err)
		assert.Equal(t, "str", obj.text)
	})

	t.Run("object list by glob", func(t *testing.T) {
		objects, err := GetObjectList[valueHolder]("holder[23]")
		require.NoError(t, err)
		assert.Len(t, objects, 2)
	})

	t.Run("object map by glob", func(t *testing.T) {
		objects, err := GetObjectMap[valueHolder]("holder[23]")
		require.NoError(t, err)
		assert.Len(t, objects, 2)
		assert.Contains(t, objects, "holder2")
		assert.Contains(t, objects, "holder3")
	})

	t.Run("multiple matches for single object", func(t *testing.T) {
		_, err := GetObject[valueHolder]("holder*")
		assert.ErrorContains(t, err, "multiple objects found")
	})

	t.Run("no match", func(t *testing.T) {
		_, err := GetObject[valueHolder]("nobody")
		assert.ErrorContains(t, err, "missing object")

		_, err = GetObjectList[valueHolder]("nobody")
		assert.ErrorContains(t, err, "missing object")

		_, err = GetObjectMap[valueHolder]("nobody")
		assert.ErrorContains(t, err, "missing object")
	})
}

func TestGetInterfaceAPIs(t *testing.T) {
	newTestContainer(t)
	Register[pluginOne](Name("plugin1"))
	Register[pluginTwo](Name("plugin2"))
	Register[pluginTwo](Name("plugin2-copy"))

	t.Run("interface by name", func(t *testing.T) {
		for name, want := range map[string]string{"plugin1": "plugin1", "plugin2": "plugin2"} {
			p, err := GetInterface[Plugin](name)
			require.NoError(t, err)
			assert.Equal(t, want, p.ID())
		}
	})

	t.Run("interface list by glob", func(t *testing.T) {
		plugins, err := GetInterfaceList[Plugin]("plugin2*")
		require.NoError(t, err)
		assert.Equal(t, []string{"plugin2", "plugin2"}, pluginIDs(plugins))
	})

	t.Run("interface map", func(t *testing.T) {
		plugins, err := GetInterfaceMap[Plugin]("*")
		require.NoError(t, err)
		require.Len(t, plugins, 3)
		assert.Equal(t, "plugin1", plugins["plugin1"].ID())
		assert.Equal(t, "plugin2", plugins["plugin2-copy"].ID())
	})

	t.Run("multiple implementations", func(t *testing.T) {
		_, err := GetInterface[Plugin]("*")
		assert.ErrorContains(t, err, "multiple implementations found")
	})
}

func TestGetOptionalObjectDirectly(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder](Optional())

	var (
		holder *valueHolder
		err    error
	)
	require.NotPanics(t, func() { holder, err = GetObject[valueHolder]("") })
	require.NoError(t, err)
	assert.Equal(t, "str", holder.text, "optional object is initialized on demand")
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestInitHook(t *testing.T) {
	newTestContainerWithConfig(t)
	Register[valueHolder]()
	Register[initHook]()

	hook, err := GetObject[initHook]("")
	require.NoError(t, err)
	assert.Equal(t, "init", hook.holder.text)
	assert.Equal(t, "str_ptr", *hook.holder.textPtr)
}

func TestInitError(t *testing.T) {
	newTestContainer(t)
	Register[failingInit]()

	for i := 0; i < 2; i++ {
		_, err := GetObject[failingInit]("")
		assert.EqualError(t, err, "init failed", "attempt %d must not report a circular dependency", i+1)
	}
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

func TestRegisterErrors(t *testing.T) {
	t.Run("duplicate registration", func(t *testing.T) {
		newTestContainer(t)
		Register[englishGreeter](Name("dup"))
		assert.PanicsWithError(t, `object <github.com/sakuradon99/ioc.englishGreeter@"dup"> already duplicate register`, func() {
			Register[englishGreeter](Name("dup"))
		})
	})

	t.Run("non struct type", func(t *testing.T) {
		newTestContainer(t)
		assert.Panics(t, func() { Register[int]() })
	})

	t.Run("unsupported inject field type", func(t *testing.T) {
		newTestContainer(t)
		assert.PanicsWithError(t, "unsupported inject field type [int]", func() {
			Register[unsupportedInjectField]()
		})
	})

	t.Run("constructor returning another type", func(t *testing.T) {
		newTestContainer(t)
		assert.Panics(t, func() {
			Register[holderRef](Constructor(func() *englishGreeter { return nil }))
		})
	})

	t.Run("constructor is not a function", func(t *testing.T) {
		newTestContainer(t)
		assert.Panics(t, func() { Register[holderRef](Constructor(42)) })
	})
}

func TestResolveErrors(t *testing.T) {
	t.Run("missing implementation", func(t *testing.T) {
		newTestContainer(t)
		Register[requiredDependency]()

		_, err := GetObject[requiredDependency]("")
		assert.ErrorContains(t, err, "missing implementation")
	})

	t.Run("missing object", func(t *testing.T) {
		newTestContainer(t)
		Register[requiredObjectDependency]()

		_, err := GetObject[requiredObjectDependency]("")
		assert.ErrorContains(t, err, "missing object")
	})

	t.Run("missing required value", func(t *testing.T) {
		newTestContainer(t)
		Register[mapValueTarget]()

		_, err := GetObject[mapValueTarget]("")
		assert.EqualError(t, err, "missing value <key1>")
	})

	t.Run("condition not returning bool", func(t *testing.T) {
		newTestContainer(t)
		Register[pluginOne](Conditional("1 + 1"))

		_, err := GetInterface[Plugin]("")
		assert.ErrorContains(t, err, "not return bool")
	})

	t.Run("duplicate names in map", func(t *testing.T) {
		newTestContainer(t)
		Register[pluginOne](Name("same"))
		Register[pluginTwo](Name("same"))

		_, err := GetInterfaceMap[Plugin]("*")
		assert.ErrorContains(t, err, `duplicate name`)
	})
}

func TestTypeKindErrors(t *testing.T) {
	newTestContainer(t)

	obj, err := GetObject[int]("")
	assert.Nil(t, obj)
	assert.EqualError(t, err, "ref is not a struct")

	objList, err := GetObjectList[int]("")
	assert.Nil(t, objList)
	assert.EqualError(t, err, "ref is not a struct")

	objMap, err := GetObjectMap[int]("")
	assert.Nil(t, objMap)
	assert.EqualError(t, err, "ref is not a struct")

	itf, err := GetInterface[valueHolder]("")
	assert.Equal(t, valueHolder{}, itf)
	assert.EqualError(t, err, "ref is not a interface")

	itfList, err := GetInterfaceList[valueHolder]("")
	assert.Nil(t, itfList)
	assert.EqualError(t, err, "ref is not a interface")

	itfMap, err := GetInterfaceMap[valueHolder]("")
	assert.Nil(t, itfMap)
	assert.EqualError(t, err, "ref is not a interface")
}

func TestAddNilValueProvider(t *testing.T) {
	newTestContainer(t)
	assert.EqualError(t, AddValueProvider(nil), "provider cannot be nil")
}

func TestPrintErrorInjectionStack(t *testing.T) {
	t.Run("non injection error", func(t *testing.T) {
		assert.Empty(t, PrintErrorInjectionStack(nil))
		assert.Empty(t, PrintErrorInjectionStack(errors.New("normal error")))
	})

	t.Run("circular dependency", func(t *testing.T) {
		newTestContainer(t)
		Register[circularA]()
		Register[circularB]()

		a, err := GetObject[circularA]("")
		assert.Nil(t, a)
		require.EqualError(t, err, "circular dependency detected")

		assert.Equal(t,
			"** github.com/sakuradon99/ioc.circularA@\"\"\n"+
				"-> github.com/sakuradon99/ioc.circularB@\"\"\n"+
				"~> github.com/sakuradon99/ioc.circularA@\"\"",
			PrintErrorInjectionStack(err))
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertBasicProps(t *testing.T, props basicProps) {
	t.Helper()
	assert.Equal(t, "str", props.text)
	assert.Equal(t, "str_ptr", *props.textPtr)
	assert.Nil(t, props.missingText)
	assert.Equal(t, 1, props.count)
	assert.Equal(t, float32(0.99), props.ratio)
	assert.True(t, props.enabled)
	assert.Equal(t, []string{"str_1", "str_2"}, props.textList)
	assert.Equal(t, []string{"str_ptr_1", "str_ptr_2"}, derefAll(props.textPtrList))
}

func derefAll(ptrs []*string) []string {
	out := make([]string, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}

func pluginIDs(plugins []Plugin) []string {
	out := make([]string, len(plugins))
	for i, p := range plugins {
		out[i] = p.ID()
	}
	return out
}
