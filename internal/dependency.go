package ioc

import "reflect"

type Dependency interface {
	NameExpr() string
	RType() reflect.Type
	Optional() bool
	FullType() string
	isInterface() bool
}

type valueDependency struct {
	keyExpr  string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newValueDependency(keyExpr string, rtp reflect.Type, optional bool) *valueDependency {
	return &valueDependency{
		keyExpr:  keyExpr,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
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

func (d *valueDependency) FullType() string {
	return d.fullType
}

func (d *valueDependency) isInterface() bool {
	return false
}

type objectDependency struct {
	nameExpr string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newObjectDependency(nameExpr string, rtp reflect.Type, optional bool) *objectDependency {
	return &objectDependency{
		nameExpr: nameExpr,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *objectDependency) NameExpr() string {
	return d.nameExpr
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

type objectListDependency struct {
	nameExpr string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newObjectListDependency(nameExpr string, rtp reflect.Type, optional bool) *objectListDependency {
	return &objectListDependency{
		nameExpr: nameExpr,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *objectListDependency) NameExpr() string {
	return d.nameExpr
}

func (d *objectListDependency) RType() reflect.Type {
	return d.rtp
}

func (d *objectListDependency) Optional() bool {
	return d.optional
}

func (d *objectListDependency) FullType() string {
	return d.fullType
}

func (d *objectListDependency) isInterface() bool {
	return d.rtp.Kind() == reflect.Interface
}

func (d *objectListDependency) SliceType() reflect.Type {
	if d.RType().Kind() == reflect.Interface {
		return reflect.SliceOf(d.RType())
	}
	return reflect.SliceOf(reflect.New(d.rtp).Type())
}

type objectMapDependency struct {
	nameExpr string
	rtp      reflect.Type
	optional bool
	fullType string
}

func newObjectMapDependency(nameExpr string, rtp reflect.Type, optional bool) *objectMapDependency {
	return &objectMapDependency{
		nameExpr: nameExpr,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
	}
}

func (d *objectMapDependency) NameExpr() string {
	return d.nameExpr
}

func (d *objectMapDependency) RType() reflect.Type {
	return d.rtp
}

func (d *objectMapDependency) Optional() bool {
	return d.optional
}

func (d *objectMapDependency) FullType() string {
	return d.fullType
}

func (d *objectMapDependency) isInterface() bool {
	return d.rtp.Kind() == reflect.Interface
}

func (d *objectMapDependency) MapType() reflect.Type {
	if d.RType().Kind() == reflect.Interface {
		return reflect.MapOf(reflect.TypeOf(""), d.RType())
	}
	return reflect.MapOf(reflect.TypeOf(""), reflect.New(d.RType()).Type())
}
