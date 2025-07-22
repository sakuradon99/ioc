package ioc

import (
	"fmt"
	"reflect"
)

type InstanceBuilder interface {
	Build(args []any) (any, error)
}

type fieldInstanceBuilder struct {
	ot                 reflect.Type
	injectFieldIndexes [][]int
}

func newFieldInstanceBuilder(ot reflect.Type, injectFieldIndexes [][]int) *fieldInstanceBuilder {
	return &fieldInstanceBuilder{ot: ot, injectFieldIndexes: injectFieldIndexes}
}

func (b *fieldInstanceBuilder) Build(args []any) (any, error) {
	ov := reflect.New(b.ot)
	oe := ov.Elem()

	var fn func(fv reflect.Value, arg any, fIndex []int)
	fn = func(fv reflect.Value, arg any, fIndex []int) {
		if len(fIndex) == 0 {
			f := newStructField(fv)
			f.Assign(arg)
		} else {
			fn(fv.Field(fIndex[0]), arg, fIndex[1:])
		}
	}

	for index, arg := range args {
		fn(oe, arg, b.injectFieldIndexes[index])
	}
	return ov.Interface(), nil
}

type constructorInstanceBuilder struct {
	constructor      any
	injectArgIndexes [][]int
}

func newConstructorInstanceBuilder(constructor any, injectArgIndexes [][]int) *constructorInstanceBuilder {
	return &constructorInstanceBuilder{constructor: constructor, injectArgIndexes: injectArgIndexes}
}

func (b *constructorInstanceBuilder) Build(args []any) (any, error) {
	ct := reflect.TypeOf(b.constructor)
	cv := reflect.ValueOf(b.constructor)

	if ct.Kind() != reflect.Func {
		return nil, newUnsupportedConstructorError(b.constructor)
	}

	var fn func(fv reflect.Value, arg any, fIndex []int)
	fn = func(fv reflect.Value, arg any, fIndex []int) {
		if len(fIndex) == 0 {
			f := newStructField(fv)
			f.Assign(arg)
		} else {
			fn(fv.Field(fIndex[0]), arg, fIndex[1:])
		}
	}

	incomes := make([]reflect.Value, ct.NumIn())
	for i := range incomes {
		incomes[i] = reflect.New(ct.In(i)).Elem()
	}
	for i, arg := range args {
		indexes := b.injectArgIndexes[i]
		if len(indexes) == 0 {
			return nil, fmt.Errorf("invalid inject arg indexes")
		}
		index := indexes[0]
		if len(indexes) == 1 {
			incomes[index].Set(reflect.ValueOf(arg))
			continue
		}
		fn(incomes[index], arg, indexes[1:])
	}

	outcomes := cv.Call(incomes)
	if len(outcomes) == 2 && !outcomes[1].IsNil() {
		return nil, outcomes[1].Interface().(error)
	}

	return outcomes[0].Interface(), nil
}
