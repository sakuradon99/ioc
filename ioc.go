package ioc

import (
	"errors"
	ioc "github.com/sakuradon99/ioc/internal"
	"reflect"
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
	ret := obj.(*T)
	return ret, nil
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

	ret := make([]*T, len(objs))
	for i, obj := range objs {
		ret[i] = obj.(*T)
	}
	return ret, nil
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

	ret := make(map[string]*T, len(nameToObject))
	for k, obj := range nameToObject {
		ret[k] = obj.(*T)
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

func GetInterfaceList[T any](name string) ([]T, error) {
	rtp := getRefType[T]()
	if rtp.Kind() != reflect.Interface {
		return nil, errors.New("ref is not a interface")
	}

	objs, err := iocContainer.GetObjectList(name, rtp)
	if err != nil {
		return nil, err
	}

	ret := make([]T, len(objs))
	for i, obj := range objs {
		ret[i] = obj.(T)
	}
	return ret, nil
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

	ret := make(map[string]T, len(objs))
	for k, obj := range objs {
		ret[k] = obj.(T)
	}
	return ret, nil
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

	return val.(T), ok, nil
}
