package ioc

import (
	"reflect"
	"unsafe"
)

type Field interface {
	Type() reflect.Type
	Assign(val any)
	Append(val any)
}

type fieldImpl struct {
	v reflect.Value
}

func newFieldImpl(v reflect.Value) *fieldImpl {
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	return &fieldImpl{v: v}
}

func (f *fieldImpl) Type() reflect.Type {
	return f.v.Type()
}

func (f *fieldImpl) Assign(val any) {
	if val == nil {
		return
	}
	f.v.Set(reflect.ValueOf(val))
}

func (f *fieldImpl) Append(val any) {
	if val == nil {
		return
	}
	f.v.Set(reflect.Append(f.v, reflect.ValueOf(val)))
}
