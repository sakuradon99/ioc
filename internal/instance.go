package ioc

import (
	"fmt"
	"reflect"
)

type InstanceBuilder interface {
	Build(args []any) (any, error)
}

type constructorInstanceBuilder struct {
	constructor any
}

func newConstructorInstanceBuilder(constructor any) *constructorInstanceBuilder {
	return &constructorInstanceBuilder{constructor: constructor}
}

func (b *constructorInstanceBuilder) Build(args []any) (any, error) {
	ot := reflect.TypeOf(b.constructor)
	ov := reflect.ValueOf(b.constructor)

	if ot.Kind() != reflect.Func {
		return nil, fmt.Errorf("unsupported constructor type %s", ot.Kind())
	}

	incomes := make([]reflect.Value, ot.NumIn())
	for i, arg := range args {
		incomes[i] = reflect.ValueOf(arg)
	}

	outcomes := ov.Call(incomes)
	if len(outcomes) == 2 && !outcomes[1].IsNil() {
		return nil, outcomes[1].Interface().(error)
	}

	return outcomes[0].Interface(), nil
}

type fieldInstanceBuilder struct {
	ot                 reflect.Type
	injectFieldIndexes []int
}

func newFieldInstanceBuilder(ot reflect.Type, injectFieldIndexes []int) *fieldInstanceBuilder {
	return &fieldInstanceBuilder{ot: ot, injectFieldIndexes: injectFieldIndexes}
}

func (b *fieldInstanceBuilder) Build(args []any) (any, error) {
	ov := reflect.New(b.ot)
	for index, arg := range args {
		field := newFieldImpl(ov.Elem().Field(b.injectFieldIndexes[index]))
		field.Assign(arg)
	}
	return ov.Interface(), nil
}
