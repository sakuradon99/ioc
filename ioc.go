package ioc

import (
	"errors"
	ioc "github.com/sakuradon99/ioc/internal"
	"reflect"
)

var iocContainer ioc.Container = ioc.NewContainerImpl()

func Register[T any](opts ...ioc.RegisterOption) any {
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
	ret := obj.(*T)
	return ret, nil
}

func GetObjects[T any](name string) ([]*T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Struct {
		return nil, errors.New("ref is not a struct")
	}

	objs, err := iocContainer.GetObjects(name, rtp)
	if err != nil {
		return nil, err
	}

	ret := make([]*T, len(objs))
	for i, obj := range objs {
		ret[i] = obj.(*T)
	}
	return ret, nil
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
	ret = obj.(T)
	return ret, nil
}

func GetInterfaces[T any](name string) ([]T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Interface {
		return nil, errors.New("ref is not a interface")
	}

	objs, err := iocContainer.GetObjects(name, rtp)
	if err != nil {
		return nil, err
	}

	ret := make([]T, len(objs))
	for i, obj := range objs {
		ret[i] = obj.(T)
	}
	return ret, nil
}

func SetSourceFile(file string) {
	ioc.SourceFile = file
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

	return val.(T), ok, nil
}

func SetValue(key string, val any) {
	iocContainer.SetValue(key, val)
}

func getRefType[T any]() reflect.Type {
	return reflect.TypeOf(new(T)).Elem()
}
