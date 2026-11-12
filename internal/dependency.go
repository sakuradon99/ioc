package ioc

import "reflect"

type Dependency interface {
	NameExpr() string
	RType() reflect.Type
	Optional() bool
	FullType() string
	// Location describes where the dependency is declared, e.g. `field repo` or `constructor param #0`.
	Location() string
	isInterface() bool
}

type baseDependency struct {
	nameExpr string
	rtp      reflect.Type
	optional bool
	fullType string
	location string
}

func newBaseDependency(nameExpr string, rtp reflect.Type, optional bool, location string) baseDependency {
	return baseDependency{
		nameExpr: nameExpr,
		rtp:      rtp,
		optional: optional,
		fullType: generateFullType(rtp),
		location: location,
	}
}

func (d *baseDependency) NameExpr() string    { return d.nameExpr }
func (d *baseDependency) RType() reflect.Type { return d.rtp }
func (d *baseDependency) Optional() bool      { return d.optional }
func (d *baseDependency) FullType() string    { return d.fullType }
func (d *baseDependency) Location() string    { return d.location }
func (d *baseDependency) isInterface() bool   { return d.rtp.Kind() == reflect.Interface }

type valueDependency struct {
	baseDependency
	defaultValue    string
	hasDefaultValue bool
}

func newValueDependency(tag ValueTag, rtp reflect.Type, location string) *valueDependency {
	defaultValue, hasDefaultValue := tag.Default()
	return &valueDependency{
		baseDependency:  newBaseDependency(tag.Value(), rtp, tag.Optional(), location),
		defaultValue:    defaultValue,
		hasDefaultValue: hasDefaultValue,
	}
}

type objectDependency struct {
	baseDependency
}

func newObjectDependency(nameExpr string, rtp reflect.Type, optional bool, location string) *objectDependency {
	return &objectDependency{newBaseDependency(nameExpr, rtp, optional, location)}
}

type objectListDependency struct {
	baseDependency
}

func newObjectListDependency(nameExpr string, rtp reflect.Type, optional bool, location string) *objectListDependency {
	return &objectListDependency{newBaseDependency(nameExpr, rtp, optional, location)}
}

func (d *objectListDependency) SliceType() reflect.Type {
	if d.isInterface() {
		return reflect.SliceOf(d.rtp)
	}
	return reflect.SliceOf(reflect.PointerTo(d.rtp))
}

type objectMapDependency struct {
	baseDependency
}

func newObjectMapDependency(nameExpr string, rtp reflect.Type, optional bool, location string) *objectMapDependency {
	return &objectMapDependency{newBaseDependency(nameExpr, rtp, optional, location)}
}

func (d *objectMapDependency) MapType() reflect.Type {
	if d.isInterface() {
		return reflect.MapOf(reflect.TypeOf(""), d.rtp)
	}
	return reflect.MapOf(reflect.TypeOf(""), reflect.PointerTo(d.rtp))
}
