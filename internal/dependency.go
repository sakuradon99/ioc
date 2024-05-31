package ioc

import "reflect"

type Dependency interface {
	ID() string
	IsInterface() bool
	Optional() bool
	Name() string
	Type() reflect.Type
}

type dependencyImpl struct {
	name     string
	optional bool
	t        reflect.Type
}

func newDependencyImpl(name string, optional bool, t reflect.Type) *dependencyImpl {
	return &dependencyImpl{name: name, optional: optional, t: t}
}

func (d *dependencyImpl) ID() string {
	if d.IsInterface() {
		return generateFullType(d.t)
	}
	return generateObjectID(d.t, d.name)
}

func (d *dependencyImpl) IsInterface() bool {
	return d.t.Kind() == reflect.Interface
}

func (d *dependencyImpl) Optional() bool {
	return d.optional
}

func (d *dependencyImpl) Name() string {
	return d.name
}

func (d *dependencyImpl) Type() reflect.Type {
	return d.t
}
