package ioc

import (
	"reflect"
	"unsafe"
)

func assignPrivateField(field reflect.Value, val any) {
	field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	if v, ok := val.(reflect.Value); ok {
		field.Set(v)
		return
	}
	field.Set(reflect.ValueOf(val))
}
