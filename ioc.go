package ioc

import (
	"errors"
	"fmt"
	"reflect"

	ioc "github.com/sakuradon99/ioc/internal"
)

var iocContainer ioc.Container = ioc.NewContainerImpl()

func Register[T any](opts ...RegisterOption) any {
	err := iocContainer.Register(getRefType[T](), opts...)
	if err != nil {
		panic(err)
	}

	return nil
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
		return ret, errors.New("ref is not a interface")
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
		return nil, errors.New("ref is not a interface")
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
		return nil, errors.New("ref is not a interface")
	}
	objs, err := iocContainer.GetObjectMap(name, rtp)
	if err != nil {
		return nil, err
	}

	return castMap[T](objs)
}

func AddValueProvider(provider ValueProvider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	iocContainer.AddValueProvider(provider)
	return nil
}

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

func PrintErrorInjectionStack(err error) string {
	if err == nil {
		return ""
	}

	injectionStack, ok := err.(ioc.InjectionStack)
	if !ok {
		return ""
	}

	return injectionStack.Stack()
}
