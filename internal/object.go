package ioc

import (
	"fmt"
	"reflect"
)

type Object struct {
	id              string
	alisa           string
	typeID          string
	dependencies    []Dependency
	instanceBuilder InstanceBuilder
	ref             any
	instance        any
	inited          bool
	initializing    bool
	optional        bool
	condition       string
}

func NewObject(
	id string,
	alisa string,
	typeID string,
	condition string,
	optional bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
	ref any,
) *Object {
	return &Object{
		id:              id,
		alisa:           alisa,
		typeID:          typeID,
		condition:       condition,
		optional:        optional,
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
		ref:             ref,
	}
}

type ObjectBuilder interface {
	Build(ref any, options RegisterOptions) (*Object, error)
}

type FieldsObjectBuilder struct {
}

func NewFieldsObjectBuilder() *FieldsObjectBuilder {
	return &FieldsObjectBuilder{}
}

func (f *FieldsObjectBuilder) Build(ref any, options RegisterOptions) (*Object, error) {
	ot, typeID, objectID, err := parseObjectRef(ref, options.Alisa)
	if err != nil {
		return nil, err
	}

	var injectFieldIndexes []int
	var dependencies []Dependency
	for i := 0; i < ot.NumField(); i++ {
		field := ot.Field(i)
		if injectTagExpr, ok := field.Tag.Lookup("inject"); ok {
			injectTag := ParseInjectTag(injectTagExpr)
			injectFieldIndexes = append(injectFieldIndexes, i)
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			} else if ft.Kind() != reflect.Interface {
				return nil, fmt.Errorf(
					"unsupported inject field type <%s>, only pointer and interface type are allowed",
					ft.Kind(),
				)
			}

			dependencies = append(dependencies, NewDependencyImpl(injectTag.Value(), injectTag.Optional(), ft))
		}
	}
	obj := NewObject(
		objectID,
		options.Alisa,
		typeID,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		NewFieldInstanceBuilder(ot, injectFieldIndexes),
		ref,
	)

	return obj, nil
}

type ConstructorObjectBuilder struct {
	constructor any
}

func NewConstructorObjectBuilder() *ConstructorObjectBuilder {
	return &ConstructorObjectBuilder{}
}

func (c *ConstructorObjectBuilder) Build(ref any, options RegisterOptions) (*Object, error) {
	_, typeID, objectID, err := parseObjectRef(ref, options.Alisa)
	if err != nil {
		return nil, err
	}

	ct := reflect.TypeOf(options.Constructor)
	if ct.Kind() != reflect.Func {
		return nil, fmt.Errorf("unsupported constructor type %s", ct.Kind())
	}
	if ct.NumOut() > 2 || ct.NumOut() == 0 ||
		ct.NumOut() == 2 && ct.Out(1).Name() != "error" {
		return nil, fmt.Errorf("unsupported constructor")
	}

	var dependencies []Dependency
	for i := 0; i < ct.NumIn(); i++ {
		pt := ct.In(i)
		if pt.Kind() == reflect.Ptr {
			pt = pt.Elem()
		} else if pt.Kind() != reflect.Interface {
			return nil, fmt.Errorf(
				"unsupported constructor param type <%s>, only pointer and interface type are allowed",
				pt.Kind(),
			)
		}

		dependencies = append(dependencies, NewDependencyImpl("", false, pt))
	}
	obj := NewObject(
		objectID,
		options.Alisa,
		typeID,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		NewConstructorInstanceBuilder(options.Constructor),
		ref,
	)

	return obj, nil
}

type ObjectBuilderFactory struct {
	fieldsObjectBuilder      *FieldsObjectBuilder
	constructorObjectBuilder *ConstructorObjectBuilder
}

func NewObjectBuilderFactory() *ObjectBuilderFactory {
	return &ObjectBuilderFactory{
		fieldsObjectBuilder:      NewFieldsObjectBuilder(),
		constructorObjectBuilder: NewConstructorObjectBuilder(),
	}
}

func (f *ObjectBuilderFactory) GetBuilder(options RegisterOptions) ObjectBuilder {
	if options.Constructor != nil {
		return f.constructorObjectBuilder
	}
	return f.fieldsObjectBuilder
}

type ObjectPool struct {
	idToObject                    map[string]*Object
	interfaceIDToObjectTypeToImpl map[string]map[string]bool
	conditionExecutor             ConditionExecutor
}

func NewObjectPool(conditionExecutor ConditionExecutor) *ObjectPool {
	return &ObjectPool{
		idToObject:                    make(map[string]*Object),
		interfaceIDToObjectTypeToImpl: make(map[string]map[string]bool),
		conditionExecutor:             conditionExecutor,
	}
}

func (p *ObjectPool) Add(object *Object) error {
	if _, ok := p.idToObject[object.id]; ok {
		return fmt.Errorf("object with id %s already exists", object.id)
	}
	p.idToObject[object.id] = object
	return nil
}

func (p *ObjectPool) List() []*Object {
	var objects []*Object
	for _, obj := range p.idToObject {
		objects = append(objects, obj)
	}
	return objects
}

func (p *ObjectPool) Get(id string) (*Object, error) {
	obj, ok := p.idToObject[id]
	if !ok {
		return nil, nil
	}

	checked, err := p.checkObjectCondition(obj)
	if err != nil || !checked {
		return nil, err
	}

	return obj, nil
}

func (p *ObjectPool) GetObjectsByInterface(interfaceType reflect.Type) ([]*Object, error) {
	if interfaceType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("unsupported interface type <%s>", interfaceType.Kind())
	}

	interfaceID := generateTypeID(interfaceType)
	if _, ok := p.interfaceIDToObjectTypeToImpl[interfaceID]; !ok {
		p.interfaceIDToObjectTypeToImpl[interfaceID] = make(map[string]bool)
	}

	var implObjects []*Object
	for _, obj := range p.idToObject {
		impl, ok := p.interfaceIDToObjectTypeToImpl[interfaceID][obj.typeID]
		if ok {
			if impl {
				checked, err := p.checkObjectCondition(obj)
				if err != nil {
					return nil, err
				}
				if checked {
					implObjects = append(implObjects, obj)
				}
			}
			continue
		}

		if obj.ref == nil {
			return nil, fmt.Errorf("object <%s> ref is nil", obj.id)
		}
		if !reflect.TypeOf(obj.ref).Implements(interfaceType) {
			p.interfaceIDToObjectTypeToImpl[interfaceID][obj.typeID] = false
			continue
		}

		checked, err := p.checkObjectCondition(obj)
		if err != nil {
			return nil, err
		}
		if !checked {
			continue
		}
		implObjects = append(implObjects, obj)
		p.interfaceIDToObjectTypeToImpl[interfaceID][obj.typeID] = true
	}

	return implObjects, nil
}

func (p *ObjectPool) checkObjectCondition(obj *Object) (bool, error) {
	if obj.condition == "" {
		return true, nil
	}
	return p.conditionExecutor.Execute(obj.condition)
}

func generateObjectID(t reflect.Type, alisa string) string {
	return fmt.Sprintf("%s-%s", generateTypeID(t), alisa)
}

func generateTypeID(t reflect.Type) string {
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

func parseObjectRef(ref any, alisa string) (reflect.Type, string, string, error) {
	rt := reflect.TypeOf(ref)
	if rt.Kind() != reflect.Ptr {
		return nil, "", "", fmt.Errorf("unsupported object ref type <%s>, must be pointer", rt.Kind())
	}
	rt = rt.Elem()
	typeID := fmt.Sprintf("%s.%s", rt.PkgPath(), rt.Name())
	objectID := typeID + "-" + alisa

	return rt, typeID, objectID, nil
}
