package ioc

import (
	"reflect"
	"unsafe"
)

func assignPrivateField(field reflect.Value, val any) {
	if val == nil {
		return
	}
	field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	field.Set(reflect.ValueOf(val))
}
