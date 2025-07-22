# IOC Container for Go

This project provides an Inversion of Control (IOC) container implementation for Go, enabling dependency injection and object management. It simplifies the creation, registration, and retrieval of objects and values in your application.

## Features

- **Object Registration**: Register objects with options like name, alias, constructor, and conditional expressions.
- **Dependency Injection**: Automatically inject dependencies into constructors or fields using `inject` tags.
- **Object Retrieval**: Retrieve objects, lists, or maps by name or type.
- **Value Management**: Add and retrieve values using a value provider and `value` tags.
- **Optional Dependencies**: Mark dependencies as optional to avoid panics when missing.

## Installation

Use `go get` to install the package:

```bash
go get github.com/sakuradon99/ioc
```

## Usage

### Registering Objects

You can register objects with various options:

```go
ioc.Register[MyService](
    ioc.Name("myService"),
    ioc.Alias("serviceAlias1", "serviceAlias2"),
    ioc.Constructor(func() *MyService { return &MyService{} }),
    ioc.Conditional("#app.name == 'myApp'"),
)
```

### Retrieving Objects

Retrieve objects by name or type:

```go
service, err := ioc.GetObject[MyService]("myService")
if err != nil {
    // Handle error
}
```

Retrieve lists or maps of objects:

```go
services, err := ioc.GetObjectList[MyService]("service*")
serviceMap, err := ioc.GetObjectMap[MyService]("service*")
```

Retrieve interfaces:

```go
myInterface, err := ioc.GetInterface[MyInterface]("myInterface")
interfaceList, err := ioc.GetInterfaceList[MyInterface]("interface*")
interfaceMap, err := ioc.GetInterfaceMap[MyInterface]("interface*")
```

### Managing Values

Add a value provider:

```go
err := ioc.AddValueProvider(myValueProvider)
```

Retrieve values:

```go
value, ok, err := ioc.GetValue[string]("app.name")
```

### Optional Dependencies

Mark dependencies as optional during registration:

```go
ioc.Register[MyService](ioc.Optional())
```

### Inject Tag

The `inject` tag is used to specify dependencies for fields in a struct. It supports name expressions, alias expressions, and glob patterns.

Example:

```go
type MyController struct {
    service *MyService `inject:"myService"`
    services []*MyService `inject:"service*"`
    serviceMap map[string]*MyService `inject:"service*"`
}
```

- **Name Expression**: Injects an object by its name.
- **Alias Expression**: Injects an object by its alias.
- **Glob Pattern**: Matches multiple objects or aliases.

If the `inject` tag matches multiple objects, they will be injected into slices or maps. For maps, the key will always be the object's name, not its alias.

### Value Tag

The `value` tag is used to inject configuration values into fields. It retrieves values from the registered value providers.

Example:

```go
type AppConfig struct {
    appName string `value:"app.name"`
    port    int    `value:"app.port"`
}
```

- **Key**: Specifies the key to retrieve the value.
- **Type**: Automatically converts the value to the field's type.

If the key is not found, the field will be set to its zero value.

## API Reference

### Registration Options

- **`Name(name string)`**: Sets the name expression for the object.
- **`Alias(aliases ...string)`**: Sets alias expressions for the object.
- **`Optional()`**: Marks the object as optional.
- **`Constructor(constructor any)`**: Sets the constructor function for the object.
- **`Conditional(expr string)`**: Sets a condition expression for the object.

### Object Retrieval

- **`GetObject[T any](name string)`**: Retrieves an object by name.
- **`GetObjectList[T any](name string)`**: Retrieves a list of objects by name.
- **`GetObjectMap[T any](name string)`**: Retrieves a map of objects by name.
- **`GetInterface[T any](name string)`**: Retrieves an interface by name.
- **`GetInterfaceList[T any](name string)`**: Retrieves a list of interfaces by name.
- **`GetInterfaceMap[T any](name string)`**: Retrieves a map of interfaces by name.

### Value Management

- **`AddValueProvider(provider ValueProvider)`**: Adds a value provider.
- **`GetValue[T any](key string)`**: Retrieves a value by key.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.