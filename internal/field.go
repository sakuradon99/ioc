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

type structField struct {
	v reflect.Value
}

func newStructField(v reflect.Value) *structField {
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	return &structField{v: v}
}

func (f *structField) Type() reflect.Type {
	return f.v.Type()
}

func (f *structField) Assign(val any) {
	// Will always assign a value to the field, even if it is nil.
	if val == nil {
		f.v.Set(reflect.Zero(f.v.Type()))
		return
	}
	if f.v.Kind() == reflect.Slice {
		vv := reflect.ValueOf(val)
		for i := 0; i < vv.Len(); i++ {
			f.v.Set(reflect.Append(f.v, vv.Index(i)))
		}
		return
	}
	f.v.Set(reflect.ValueOf(val))
}

func (f *structField) Append(val any) {
	if val == nil {
		return
	}
	f.v.Set(reflect.Append(f.v, reflect.ValueOf(val)))
}
