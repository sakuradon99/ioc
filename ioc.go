package ioc

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	ioc "github.com/sakuradon99/ioc/internal"
)

var iocContainer ioc.Container = ioc.NewContainerImpl()

var (
	// ErrNotStarted is returned when getting objects or values before Start.
	ErrNotStarted = ioc.ErrNotStarted
	// ErrStarting is returned when calling the container during Start, e.g. from Init.
	ErrStarting = ioc.ErrStarting
	// ErrAlreadyStarted is returned when registering objects or value providers after Start.
	ErrAlreadyStarted = ioc.ErrAlreadyStarted
	// ErrShutdown is returned when using the container after Shutdown.
	ErrShutdown = ioc.ErrShutdown
)

// Lifecycle hooks. An object may implement either variant of Init and Destroy.
type (
	// Initializer is called after the object is created and injected, during Start.
	Initializer = ioc.Initializer
	// ContextInitializer is like Initializer, receiving the context passed to Start.
	ContextInitializer = ioc.ContextInitializer
	// Destroyer is called on Shutdown, in reverse initialization order.
	Destroyer = ioc.Destroyer
	// ContextDestroyer is like Destroyer, receiving the context passed to Shutdown.
	ContextDestroyer = ioc.ContextDestroyer
)

// Register registers the struct type T. It panics on invalid registration or after Start.
// It returns nil so that it can be called at package level: `var _ = ioc.Register[MyService]()`.
func Register[T any](opts ...RegisterOption) any {
	err := iocContainer.Register(getRefType[T](), opts...)
	if err != nil {
		panic(err)
	}

	return nil
}

// Provide registers an existing instance. It panics on invalid instance or after Start.
//
// The instance is registered by its dynamic type, even if it is passed as an interface:
// a pointer to struct is registered as the struct type, so it can be injected into `*T` fields
// and the interfaces it implements; other types (e.g. func types) can only be injected through interfaces.
// Its fields are not injected, and the container doesn't call its Init and Destroy.
func Provide(instance any, opts ...RegisterOption) any {
	err := iocContainer.Provide(instance, opts...)
	if err != nil {
		panic(err)
	}

	return nil
}

// AddValueProvider adds a value provider. The last added provider has the highest priority.
// It must be called before Start.
func AddValueProvider(provider ValueProvider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	return iocContainer.AddValueProvider(provider)
}

// Start loads the value providers, evaluates the conditions, and initializes all non-optional objects.
// Optional objects are only initialized when depended on.
// It must be called before getting objects or values, and ctx is passed to the Init hooks.
// After Start, no more objects or value providers can be registered.
// If Start fails, the problems can be fixed and Start can be called again.
func Start(ctx context.Context) error {
	return iocContainer.Start(ctx)
}

// Shutdown calls the Destroy hooks of the initialized objects in reverse initialization order,
// and returns all the errors joined. The container can't be used after Shutdown.
func Shutdown(ctx context.Context) error {
	return iocContainer.Shutdown(ctx)
}

// Dump describes the registered objects and their dependencies, for debugging.
func Dump() string {
	return iocContainer.Dump()
}

func GetObject[T any](name string) (*T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Struct {
		return nil, errors.New("ref is not a struct")
	}

	obj, err := iocContainer.GetObject(name, rtp)
	if err != nil {
		return nil, err
	}
	return castTo[*T](obj)
}

func GetObjectList[T any](name string) ([]*T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Struct {
		return nil, errors.New("ref is not a struct")
	}

	objs, err := iocContainer.GetObjectList(name, rtp)
	if err != nil {
		return nil, err
	}

	return castList[*T](objs)
}

func GetObjectMap[T any](name string) (map[string]*T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Struct {
		return nil, errors.New("ref is not a struct")
	}

	nameToObject, err := iocContainer.GetObjectMap(name, rtp)
	if err != nil {
		return nil, err
	}

	return castMap[*T](nameToObject)
}

func GetInterface[T any](name string) (T, error) {
	var ret T
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Interface {
		return ret, errors.New("ref is not an interface")
	}

	obj, err := iocContainer.GetObject(name, rtp)
	if err != nil {
		return ret, err
	}
	return castTo[T](obj)
}

func GetInterfaceList[T any](name string) ([]T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Interface {
		return nil, errors.New("ref is not an interface")
	}

	objs, err := iocContainer.GetObjectList(name, rtp)
	if err != nil {
		return nil, err
	}

	return castList[T](objs)
}

func GetInterfaceMap[T any](name string) (map[string]T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Interface {
		return nil, errors.New("ref is not an interface")
	}
	objs, err := iocContainer.GetObjectMap(name, rtp)
	if err != nil {
		return nil, err
	}

	return castMap[T](objs)
}

// GetValue gets a value from the value providers. It must be called after Start.
func GetValue[T any](key string) (T, bool, error) {
	var defaultVal T
	val, ok, err := iocContainer.GetValue(key, getRefType[T]())
	if err != nil {
		return defaultVal, false, err
	}
	if !ok {
		return defaultVal, false, nil
	}

	ret, err := castTo[T](val)
	if err != nil {
		return defaultVal, false, err
	}
	return ret, true, nil
}

// ErrorInjectionStack returns the injection stack of an error returned by Start, e.g.
//
//	** main.App@""
//	-> main.UserService@""
//	-> main.UserRepo@""
//
// It returns an empty string if err doesn't carry an injection stack.
func ErrorInjectionStack(err error) string {
	var injectionStack ioc.InjectionStack
	if !errors.As(err, &injectionStack) {
		return ""
	}

	return injectionStack.Stack()
}

// PrintErrorInjectionStack returns the injection stack of err.
//
// Deprecated: it doesn't print anything, use ErrorInjectionStack instead.
func PrintErrorInjectionStack(err error) string {
	return ErrorInjectionStack(err)
}

// castTo converts the value returned by the container to T, returning an error instead of panicking.
func castTo[T any](v any) (T, error) {
	var zero T
	if v == nil {
		return zero, nil
	}
	ret, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected type <%T>, want <%s>", v, getRefType[T]())
	}
	return ret, nil
}

func castList[T any](objs []any) ([]T, error) {
	ret := make([]T, len(objs))
	for i, obj := range objs {
		v, err := castTo[T](obj)
		if err != nil {
			return nil, err
		}
		ret[i] = v
	}
	return ret, nil
}

func castMap[T any](objs map[string]any) (map[string]T, error) {
	ret := make(map[string]T, len(objs))
	for k, obj := range objs {
		v, err := castTo[T](obj)
		if err != nil {
			return nil, err
		}
		ret[k] = v
	}
	return ret, nil
}
