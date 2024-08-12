package ioc

import "reflect"

type Dependency interface {
	Name() string
	Type() reflect.Type
	Optional() bool
}

type valueDependency struct {
	name     string
	rtp      reflect.Type
	optional bool
}

func newValueDependency(name string, rtp reflect.Type, optional bool) *valueDependency {
	return &valueDependency{name: name, rtp: rtp, optional: optional}
}

func (d *valueDependency) Name() string {
	return d.name
}

func (d *valueDependency) Type() reflect.Type {
	return d.rtp
}

func (d *valueDependency) Optional() bool {
	return d.optional
}

type objectDependency struct {
	name     string
	rtp      reflect.Type
	optional bool
	objectID string
}

func newObjectDependency(name string, rtp reflect.Type, optional bool) *objectDependency {
	return &objectDependency{
		name:     name,
		rtp:      rtp,
		optional: optional,
		objectID: generateObjectID(rtp, name),
	}
}

func (d *objectDependency) Name() string {
	return d.name
}

func (d *objectDependency) Type() reflect.Type {
	return d.rtp
}

func (d *objectDependency) Optional() bool {
	return d.optional
}

func (d *objectDependency) ObjectID() string {
	return d.objectID
}

type interfaceDependency struct {
	name     string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newInterfaceDependency(name string, rtp reflect.Type, optional bool) *interfaceDependency {
	return &interfaceDependency{
		name:     name,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *interfaceDependency) Name() string {
	return d.name
}

func (d *interfaceDependency) Type() reflect.Type {
	return d.rtp
}

func (d *interfaceDependency) Optional() bool {
	return d.optional
}

func (d *interfaceDependency) FullType() string {
	return d.fullType
}

type interfaceListDependency struct {
	*interfaceDependency
}

func newInterfaceListDependency(name string, rtp reflect.Type, optional bool) *interfaceListDependency {
	return &interfaceListDependency{
		interfaceDependency: newInterfaceDependency(name, rtp, optional),
	}
}
