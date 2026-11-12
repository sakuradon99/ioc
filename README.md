# IOC Container for Go

A small Inversion of Control (IOC) container for Go. Register your structs, declare dependencies with struct tags or constructors, and let the container wire everything up on startup.

## Features

- **Field and constructor injection** with `inject` tags or constructor params
- **Interface injection**: objects are registered by type, and found by the interfaces they implement
- **Names, aliases, glob patterns and primary objects** to pick among multiple candidates
- **List and map injection** of all matching objects
- **Config values** from pluggable value providers, with type conversion and default values
- **Conditional registration** based on config values
- **Lifecycle**: `Start` initializes everything and fails fast, `Shutdown` destroys in reverse order
- **Readable errors** with the failing field and the injection stack, and `Dump` for debugging

## Installation

```bash
go get github.com/sakuradon99/ioc
```

## Quick Start

```go
type UserRepo struct {
    dsn string `value:"db.dsn"`
}

type UserService struct {
    repo *UserRepo `inject:""`
}

// register at package level, or anywhere before Start
var _ = ioc.Register[UserRepo]()
var _ = ioc.Register[UserService]()

func main() {
    _ = ioc.AddValueProvider(ioc.NewFileValueProvider("config.yaml"))

    if err := ioc.Start(context.Background()); err != nil {
        log.Fatalf("%v\n%s", err, ioc.ErrorInjectionStack(err))
    }
    defer ioc.Shutdown(context.Background())

    svc, err := ioc.GetObject[UserService]()
    // ...
}
```

## Lifecycle

The container goes through three phases:

1. **Register**: `Register`, `Provide` and `AddValueProvider` are only allowed before `Start`.
2. **Start**: loads the value providers, evaluates conditions, and initializes every non-optional object, calling their `Init` hooks. All errors are returned together. If `Start` fails, you can fix the problem (e.g. register the missing object) and call `Start` again.
3. **Use**: after `Start`, the container is read-only. `Get*` and `GetValue` can only be called after a successful `Start`.

`Shutdown` calls the `Destroy` hooks of the initialized objects in reverse initialization order (dependents first), and returns all errors joined. The container can't be used after `Shutdown`.

Calling the container from an `Init` hook is not allowed and returns `ioc.ErrStarting`: declare the dependency with a tag or constructor param instead.

### Hooks

An object may implement either variant of each hook:

```go
func (s *UserService) Init() error                         { ... }
func (s *UserService) Init(ctx context.Context) error      { ... } // ctx passed to Start

func (s *UserService) Destroy() error                      { ... }
func (s *UserService) Destroy(ctx context.Context) error   { ... } // ctx passed to Shutdown
```

## Registering Objects

### Register

`Register[T]` registers a struct type `T`. The container creates a `*T` and injects its tagged fields, or calls a constructor.

```go
ioc.Register[MyService](
    ioc.Name("myService"),                       // name, used by `inject:"myService"`
    ioc.Alias("svc", "service"),                 // other names
    ioc.Primary(),                               // the default among multiple candidates
    ioc.Optional(),                              // only initialized when depended on
    ioc.Conditional("#app.mode == 'prod'"),      // only registered if the condition is true
    ioc.Constructor(NewMyService),               // create with a constructor
)
```

`Register` panics on invalid registration, and returns `nil` so it can be used as `var _ = ioc.Register[T]()`.

### Provide

`Provide` registers an existing instance, e.g. a client created by another library:

```go
ioc.Provide(db)                         // *sql.DB, registered as sql.DB
ioc.Provide(lib.NewLogger(), ioc.Name("logger")) // returns an interface, registered by its dynamic type
```

- The instance is registered by its **dynamic type**: a pointer to struct is registered as the struct type, so it can be injected into `*T` fields and the interfaces it implements. Other types (e.g. func types) can only be injected through interfaces.
- The instance must not be nil, and must not be a struct value (pass a pointer).
- Its fields are **not** injected, and the container does **not** call its `Init` and `Destroy`: the caller owns it.

### Constructor

```go
func NewMyService(repo *UserRepo, greeter Greeter, params struct {
    cache Cache  `inject:"redis"`
    port  int    `value:"server.port"`
}) (*MyService, error) {
    ...
}
```

- It must return `*T` or `(*T, error)`, where `T` is the registered type.
- Pointer and interface params are required dependencies, matched by type like `inject:""`.
- Struct params are created by the container, with their tagged fields injected, so you can use names, optional dependencies and values.

### Optional

An optional object is not initialized on `Start` unless another object depends on it, so its own dependencies may be missing as long as nobody uses it. Getting an optional object nobody depends on returns an error.

### Conditional

The condition is a [govaluate](https://github.com/Knetic/govaluate) expression evaluated once on `Start`. Use `#key` to refer to a config value:

```go
ioc.Register[RedisCache](ioc.Conditional("#cache.type == 'redis'"))
ioc.Register[MemoryCache](ioc.Conditional("#cache.type == 'memory' || #cache.type == nil"))
```

## Injection

### Inject Tag

```go
type MyController struct {
    service    *MyService            `inject:""`               // by type
    greeter    Greeter               `inject:"english"`        // interface, by name or alias
    cache      Cache                 `inject:";optional"`      // nil if missing
    services   []*MyService          `inject:"service*"`       // all matching objects
    handlers   []Handler             `inject:""`               // all implementations
    handlerMap map[string]Handler    `inject:"*"`              // keyed by object name
}
```

Supported field types: `*Struct`, `Interface`, `[]*Struct`, `[]Interface`, `map[string]*Struct`, `map[string]Interface`. Unexported fields are supported. Untagged struct fields (including embedded structs) are scanned recursively.

The tag value is a name expression, matched against the object names and aliases with [glob patterns](https://pkg.go.dev/path/filepath#Match). For maps, the key is always the object name, not the alias.

### Matching Rules

For a single object with an empty name expression (`inject:""`, constructor params, `GetObject[T]()`), candidates are matched by type:

1. the only unnamed object, otherwise
2. the only `Primary` object, otherwise
3. the only object of the type, otherwise
4. an error listing the candidates.

With a non-empty name expression, `Primary` has no effect and the expression must match exactly one object. Lists and maps get all matching objects, and an empty expression matches all objects of the type.

## Values

### Value Providers

Values come from value providers. The last added provider has the highest priority, and keys missing in it fall back to earlier providers.

```go
ioc.AddValueProvider(ioc.NewFileValueProvider("config.yaml")) // .yaml, .yml or .json
ioc.AddValueProvider(ioc.NewMapValueProvider(map[string]any{"app": map[string]any{"name": "demo"}}))
```

Implement `ValueProvider` to load values from anywhere else (environment variables, a config center...):

```go
type ValueProvider interface {
    Provide() (map[string]any, error)
}
```

Providers are loaded once on `Start`.

### Value Tag

```go
type AppConfig struct {
    name    string        `value:"app.name"`                    // required
    debug   bool          `value:"app.debug;optional"`          // zero value if missing
    port    int           `value:"server.port;default=8080"`    // default if missing
    timeout time.Duration `value:"server.timeout;default=5s"`
    tags    []string      `value:"app.tags"`
    db      DBConfig      `value:"db"`                          // struct, see below
}
```

Keys are dot-separated paths into nested maps. Values are converted to the field type:

- basic types (`string`, `bool`, ints, uints, floats) and named types based on them (`type Level int`)
- `time.Duration`, from strings like `"5s"` or from nanoseconds
- pointers, slices, arrays and `map[string]T` of supported types
- structs, from a map, using `property` tags
- `any`, as is

Conversion is strict: a value of the wrong type, an overflowing number, or a float with a fraction for an int field is an error.

### Struct Values and the Property Tag

Fields of a struct value are mapped with `property` tags, whose keys are **relative** to the struct value. Untagged embedded structs read their properties from the same level.

```go
type DBConfig struct {
    host string `property:"host;default=127.0.0.1"`
    port int    `property:"port"`
}
```

`value` and `property` are different tags on purpose: `value` keys are absolute paths, `property` keys are relative to their struct.

### Default Values

`default=` is supported by both `value` and `property` tags, for basic types, named basic types, `time.Duration` and pointers to them. It is used only when the value is missing; a present value that can't be converted is still an error. Invalid defaults are reported on `Register`. A default value can't contain `;`.

### Getting Values

```go
name, ok, err := ioc.GetValue[string]("app.name")
```

## Getting Objects

```go
svc, err := ioc.GetObject[MyService]()                    // *MyService
list, err := ioc.GetObjectList[MyService]("service*")     // []*MyService
m, err := ioc.GetObjectMap[MyService]("service*")         // map[string]*MyService

g, err := ioc.GetInterface[Greeter]("chinese", "english") // Greeter
gs, err := ioc.GetInterfaceList[Greeter]("en*", "zh*")    // []Greeter
gm, err := ioc.GetInterfaceMap[Greeter]()                 // map[string]Greeter
```

Each name follows the same rules as the `inject` tag, and passing no name is the same as passing `""`.

- `GetObject` / `GetInterface` try the names in order and return the object found by the first name that matches any. A name matching multiple objects is an error.
- The list and map variants return all the objects matched by any of the names, without duplicates.

## Errors and Debugging

Errors returned by `Start` tell which object and field failed:

```
<main.UserRepo@""> field dsn: missing value <db.dsn>
```

`ErrorInjectionStack` returns how the failing object was reached:

```go
fmt.Println(ioc.ErrorInjectionStack(err))
// ** main.App@""
// -> main.UserService@""
// -> main.UserRepo@""
```

Lifecycle errors can be checked with `errors.Is`: `ErrNotStarted`, `ErrStarting`, `ErrAlreadyStarted`, `ErrShutdown`.

`Dump` describes the registered objects and their dependencies:

```
container: started, 3 object(s)
* main.UserRepo@"" [initialized]
    field dsn <- value "db.dsn"
* main.UserService@"" [initialized, primary]
    field repo <- main.UserRepo@""
* main.RedisCache@"" [not initialized, condition="#cache.type == 'redis'" -> false]
```

## API Reference

### Lifecycle

- `Start(ctx context.Context) error`
- `Shutdown(ctx context.Context) error`

### Registration

- `Register[T any](opts ...RegisterOption) any`
- `Provide(instance any, opts ...RegisterOption) any`
- Options: `Name(name)`, `Alias(aliases...)`, `Primary()`, `Optional()`, `Constructor(fn)`, `Conditional(expr)`

### Objects

- `GetObject[T any](names ...string) (*T, error)`
- `GetObjectList[T any](names ...string) ([]*T, error)`
- `GetObjectMap[T any](names ...string) (map[string]*T, error)`
- `GetInterface[T any](names ...string) (T, error)`
- `GetInterfaceList[T any](names ...string) ([]T, error)`
- `GetInterfaceMap[T any](names ...string) (map[string]T, error)`

### Values

- `AddValueProvider(provider ValueProvider) error`
- `GetValue[T any](key string) (T, bool, error)`
- `NewFileValueProvider(file string)`, `NewMapValueProvider(values map[string]any)`

### Debugging

- `ErrorInjectionStack(err error) string`
- `Dump() string`
