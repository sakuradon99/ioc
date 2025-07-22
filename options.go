package ioc

import ioc "github.com/sakuradon99/ioc/internal"

type RegisterOption = ioc.RegisterOption

// Name sets the name expression for the registered object.
// The name expression can be used to inject the object by name, without this option, the name will be empty "".
// Example: `Name("myService")`, in the field, you can use `inject:"myService"` to inject this object.
// The name expression can be a simple string or a glob pattern.
// Example: `inject:"myService*"` will match any object whose name starts with "myService".
func Name(name string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Name = name
	}
}

// Alias sets the alias expressions for the registered object.
// The alias expressions can be used to inject the object by alias, without this option, the alias will be empty [].
// Example: `Alias("myServiceAlias1", "myServiceAlias2")`,
// in the field, you can use `inject:"myServiceAlias1"` or `inject:"myServiceAlias2"` to inject this object.
// If the inject expression is a glob pattern, and matches multiple aliases from the same object,
// this object will be injected only once.
// If inject to a map, the key will still be the name not the alias.
// Example: `map[string]Service inject:"myServiceAlias*"`,
// the map will contain the object with the name "myService" and not the alias.
func Alias(aliases ...string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Aliases = aliases
	}
}

// Optional marks the registered object as optional
// If the object is not found, will inject nil to the field instead of panic.
func Optional() ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Optional = true
	}
}

// Constructor sets the constructor function for the registered object.
// The constructor function will be called to create the object when it is requested.
// The signature of the constructor function should be:
// - `func() Object`
// - `func() (*Object, error)`
// - `func(Dependency1, Dependency2...) Object`
// - `func(Dependency1, Dependency2...) (*Object, error)`
// Example: `Constructor(func() *MyService { return &MyService{} })`
// The type of dependencies can only be pointer, interface or struct.
// If the dependency is pointer or interface, it will be injected automatically by the IOC container.
// If the dependency is a struct, the container will create a new instance of the struct and inject its fields.
func Constructor(constructor any) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Constructor = constructor
	}
}

// Conditional sets a condition expression for the registered object.
// The object will only be registered if the condition expression evaluates to true.
// You can use `#` to refer the provided value in the condition expression.
// Example: `Conditional("#app.name == 'myApp'")`
func Conditional(expr string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.ConditionExpr = expr
	}
}
