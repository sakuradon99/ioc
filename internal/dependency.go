package ioc

import "reflect"

type Dependency interface {
	ID() string
	IsInterface() bool
	Optional() bool
	Alisa() string
	Type() reflect.Type
}

type DependencyImpl struct {
	alisa    string
	optional bool
	t        reflect.Type
}

func NewDependencyImpl(alisa string, optional bool, t reflect.Type) *DependencyImpl {
	return &DependencyImpl{alisa: alisa, optional: optional, t: t}
}

func (d *DependencyImpl) ID() string {
	if d.IsInterface() {
		return generateTypeID(d.t)
	}
	return generateObjectID(d.t, d.alisa)
}

func (d *DependencyImpl) IsInterface() bool {
	return d.t.Kind() == reflect.Interface
}

func (d *DependencyImpl) Optional() bool {
	return d.optional
}

func (d *DependencyImpl) Alisa() string {
	return d.alisa
}

func (d *DependencyImpl) Type() reflect.Type {
	return d.t
}
