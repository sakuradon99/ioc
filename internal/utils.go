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

	if field.Kind() == reflect.Slice {
		srcValue := reflect.ValueOf(val)
		for i := 0; i < srcValue.Len(); i++ {
			elem := srcValue.Index(i)
			convertedElem := elem.Convert(field.Type().Elem())
			field = reflect.Append(field, convertedElem)
		}
		return
	}
	field.Set(reflect.ValueOf(val))
}

type Field struct {
	v reflect.Value
}

func NewField(v reflect.Value) *Field {
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	return &Field{v: v}
}

func (f *Field) Type() reflect.Type {
	return f.v.Type()
}

func (f *Field) Assign(val any) {
	if val == nil {
		return
	}
	f.v.Set(reflect.ValueOf(val))
}

func (f *Field) Append(val any) {
	if val == nil {
		return
	}
	f.v.Set(reflect.Append(f.v, reflect.ValueOf(val)))
}
