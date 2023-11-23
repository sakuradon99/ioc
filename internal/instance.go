package ioc

import (
	"fmt"
	"reflect"
)

type InstanceBuilder interface {
	Build(args []any) (any, error)
}

type ConstructorInstanceBuilder struct {
	constructor any
}

func NewConstructorInstanceBuilder(constructor any) *ConstructorInstanceBuilder {
	return &ConstructorInstanceBuilder{constructor: constructor}
}

func (b *ConstructorInstanceBuilder) Build(args []any) (any, error) {
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

type FieldInstanceBuilder struct {
	ot                 reflect.Type
	injectFieldIndexes []int
}

func NewFieldInstanceBuilder(ot reflect.Type, injectFieldIndexes []int) *FieldInstanceBuilder {
	return &FieldInstanceBuilder{ot: ot, injectFieldIndexes: injectFieldIndexes}
}

func (b *FieldInstanceBuilder) Build(args []any) (any, error) {
	ov := reflect.New(b.ot)
	for index, arg := range args {
		assignPrivateField(ov.Elem().Field(index), arg)
	}
	return ov.Interface(), nil
}
