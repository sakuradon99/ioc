package ioc

import "reflect"

type Dependency interface {
	ID() string
	IsInterface() bool
	Optional() bool
	Name() string
	Type() reflect.Type
}

type DependencyImpl struct {
	name     string
	optional bool
	t        reflect.Type
}

func NewDependencyImpl(name string, optional bool, t reflect.Type) *DependencyImpl {
	return &DependencyImpl{name: name, optional: optional, t: t}
}

func (d *DependencyImpl) ID() string {
	if d.IsInterface() {
		return generateFullType(d.t)
	}
	return generateObjectID(d.t, d.name)
}

func (d *DependencyImpl) IsInterface() bool {
	return d.t.Kind() == reflect.Interface
}

func (d *DependencyImpl) Optional() bool {
	return d.optional
}

func (d *DependencyImpl) Name() string {
	return d.name
}

func (d *DependencyImpl) Type() reflect.Type {
	return d.t
}
