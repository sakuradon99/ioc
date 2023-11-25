package ioc

import (
	"fmt"
)

type Object struct {
	id              string
	name            string
	instanceBuilder InstanceBuilder
	dependencies    []Dependency
	instance        any
	inited          bool
	initializing    bool
	optional        bool
	condition       string
}

func NewObject(
	id string,
	name string,
	condition string,
	optional bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
) *Object {
	return &Object{
		id:              id,
		name:            name,
		condition:       condition,
		optional:        optional,
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
	}
}

type ObjectPool struct {
	objects           map[string]*Object
	conditionExecutor ConditionExecutor
}

func NewObjectPool(conditionExecutor ConditionExecutor) *ObjectPool {
	return &ObjectPool{
		objects:           make(map[string]*Object),
		conditionExecutor: conditionExecutor,
	}
}

func (p *ObjectPool) Add(object *Object) error {
	if _, ok := p.objects[object.id]; ok {
		return fmt.Errorf("object with id %s already exists", object.id)
	}
	p.objects[object.id] = object
	return nil
}

func (p *ObjectPool) List() []*Object {
	var objects []*Object
	for _, obj := range p.objects {
		objects = append(objects, obj)
	}
	return objects
}

func (p *ObjectPool) Get(id string) (*Object, error) {
	obj, ok := p.objects[id]
	if !ok {
		return nil, nil
	}
	if obj.condition != "" {
		ok, err := p.conditionExecutor.Execute(obj.condition)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, err
		}
	}
	return obj, nil
}

func genObjectID(pkgPath string, name string, alisa string) string {
	return fmt.Sprintf("%s.%s-%s", pkgPath, name, alisa)
}

type Dependency struct {
	isInterface bool
	pkgPath     string
	name        string
	alisa       string
	optional    bool
}
