package ioc

import (
	"errors"
	ioc "github.com/sakuradon99/ioc/internal"
	"reflect"
)

var iocContainer ioc.Container = ioc.NewContainerImpl()

func Register[T any](opts ...ioc.RegisterOption) any {
	err := iocContainer.Register(new(T), opts...)
	if err != nil {
		panic(err)
	}

	return nil
}

func GetObject[T any](name string) (*T, error) {
	if reflect.TypeOf(new(T)).Elem().Kind() != reflect.Struct {
		return nil, errors.New("ref is not a struct")
	}

	obj, err := iocContainer.GetObject(name, new(T))
	if err != nil {
		return nil, err
	}
	ret := obj.(*T)
	return ret, nil
}

func GetInterface[T any](name string) (T, error) {
	var ret T
	if reflect.TypeOf(new(T)).Elem().Kind() != reflect.Interface {
		return ret, errors.New("ref is not an interface")
	}

	obj, err := iocContainer.GetObject(name, new(T))
	if err != nil {
		return ret, err
	}
	ret = obj.(T)
	return ret, nil
}

func SetSourceFile(file string) {
	ioc.SourceFile = file
}
