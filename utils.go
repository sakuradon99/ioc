package ioc

import "reflect"

func getRefType[T any]() reflect.Type {
	return reflect.TypeOf(new(T)).Elem()
}
