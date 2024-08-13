package ioc

import "reflect"

type Dependency interface {
	NameExpr() string
	RType() reflect.Type
	Optional() bool
}

type valueDependency struct {
	keyExpr  string
	rtp      reflect.Type
	optional bool
}

func newValueDependency(keyExpr string, rtp reflect.Type, optional bool) *valueDependency {
	return &valueDependency{keyExpr: keyExpr, rtp: rtp, optional: optional}
}

func (d *valueDependency) NameExpr() string {
	return d.keyExpr
}

func (d *valueDependency) RType() reflect.Type {
	return d.rtp
}

func (d *valueDependency) Optional() bool {
	return d.optional
}

type objectDependency struct {
	name     string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newObjectDependency(name string, rtp reflect.Type, optional bool) *objectDependency {
	return &objectDependency{
		name:     name,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *objectDependency) NameExpr() string {
	return d.name
}

func (d *objectDependency) RType() reflect.Type {
	return d.rtp
}

func (d *objectDependency) Optional() bool {
	return d.optional
}

func (d *objectDependency) FullType() string {
	return d.fullType
}

func (d *objectDependency) isInterface() bool {
	return d.rtp.Kind() == reflect.Interface
}

type objectsDependency struct {
	name     string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newObjectsDependency(name string, rtp reflect.Type, optional bool) *objectsDependency {
	return &objectsDependency{
		name:     name,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *objectsDependency) NameExpr() string {
	return d.name
}

func (d *objectsDependency) RType() reflect.Type {
	return d.rtp
}

func (d *objectsDependency) Optional() bool {
	return d.optional
}

func (d *objectsDependency) FullType() string {
	return d.fullType
}

func (d *objectsDependency) SliceType() reflect.Type {
	if d.rtp.Kind() == reflect.Interface {
		return reflect.SliceOf(d.rtp)
	}
	return reflect.SliceOf(reflect.New(d.rtp).Type())
}
