package ioc

import (
	"fmt"
	"reflect"
)

type ObjectStatus int

const (
	ObjectStatusDefault ObjectStatus = iota
	ObjectStatusInitializing
	ObjectStatusInitialized
)

type Object interface {
	ObjectRef
	Name() string
	Condition() string
	Dependencies() []Dependency
	Instance() any
	Status() ObjectStatus
	StartInitialization()
	Build(args []any) (any, error)
	Optional() bool
}

type objectImpl struct {
	ObjectRef
	name            string
	dependencies    []Dependency
	instanceBuilder InstanceBuilder
	optional        bool
	condition       string
	instance        any
	inited          bool
	initializing    bool
}

func newObject(
	of ObjectRef,
	name string,
	condition string,
	optional bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
) *objectImpl {
	return &objectImpl{
		ObjectRef:       of,
		name:            name,
		condition:       condition,
		optional:        optional,
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
	}
}

func (o *objectImpl) Name() string {
	return o.name
}

func (o *objectImpl) Optional() bool {
	return o.optional
}

func (o *objectImpl) Condition() string {
	return o.condition
}

func (o *objectImpl) Dependencies() []Dependency {
	return o.dependencies
}

func (o *objectImpl) Instance() any {
	return o.instance
}

func (o *objectImpl) Status() ObjectStatus {
	if o.inited {
		return ObjectStatusInitialized
	}
	if o.initializing {
		return ObjectStatusInitializing
	}
	return ObjectStatusDefault
}

func (o *objectImpl) StartInitialization() {
	o.initializing = true
}

func (o *objectImpl) Build(args []any) (any, error) {
	instance, err := o.instanceBuilder.Build(args)
	if err != nil {
		return nil, err
	}
	o.instance = instance
	o.inited = true
	return instance, nil
}

type ObjectBuilder interface {
	Build(ref any, options RegisterOptions) (Object, error)
}

type fieldsObjectBuilder struct {
}

func newFieldsObjectBuilder() *fieldsObjectBuilder {
	return &fieldsObjectBuilder{}
}

func (f *fieldsObjectBuilder) Build(ref any, options RegisterOptions) (Object, error) {
	of, err := parseObjectRef(ref, options.Name)
	if err != nil {
		return nil, err
	}

	var injectFieldIndexes []int
	var dependencies []Dependency
	ot := of.Type()
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

			dependencies = append(dependencies, newDependencyImpl(injectTag.Value(), injectTag.Optional(), ft))
		}
	}
	obj := newObject(
		of,
		options.Name,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		newFieldInstanceBuilder(of.Type(), injectFieldIndexes),
	)

	return obj, nil
}

type constructorObjectBuilder struct {
	constructor any
}

func newConstructorObjectBuilder() *constructorObjectBuilder {
	return &constructorObjectBuilder{}
}

func (c *constructorObjectBuilder) Build(ref any, options RegisterOptions) (Object, error) {
	of, err := parseObjectRef(ref, options.Name)
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

		dependencies = append(dependencies, newDependencyImpl("", false, pt))
	}
	obj := newObject(
		of,
		options.Name,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		newConstructorInstanceBuilder(options.Constructor),
	)

	return obj, nil
}

type ObjectBuilderFactory interface {
	GetBuilder(options RegisterOptions) ObjectBuilder
}

type objectBuilderFactoryImpl struct {
	fieldsObjectBuilder      *fieldsObjectBuilder
	constructorObjectBuilder *constructorObjectBuilder
}

func newObjectBuilderFactoryImpl() *objectBuilderFactoryImpl {
	return &objectBuilderFactoryImpl{
		fieldsObjectBuilder:      newFieldsObjectBuilder(),
		constructorObjectBuilder: newConstructorObjectBuilder(),
	}
}

func (f *objectBuilderFactoryImpl) GetBuilder(options RegisterOptions) ObjectBuilder {
	if options.Constructor != nil {
		return f.constructorObjectBuilder
	}
	return f.fieldsObjectBuilder
}

type ObjectPool interface {
	Add(object Object) error
	List() []Object
	Get(id string) (Object, error)
	GetObjectsByInterface(interfaceType reflect.Type) ([]Object, error)
}

type objectPoolImpl struct {
	idToObject                    map[string]Object
	interfaceIDToObjectTypeToImpl map[string]map[string]bool
	conditionExecutor             ConditionExecutor
}

func newObjectPoolImpl(conditionExecutor ConditionExecutor) *objectPoolImpl {
	return &objectPoolImpl{
		idToObject:                    make(map[string]Object),
		interfaceIDToObjectTypeToImpl: make(map[string]map[string]bool),
		conditionExecutor:             conditionExecutor,
	}
}

func (p *objectPoolImpl) Add(object Object) error {
	if _, ok := p.idToObject[object.ObjectID()]; ok {
		return fmt.Errorf("object with id %s already exists", object.ObjectID())
	}
	p.idToObject[object.ObjectID()] = object
	return nil
}

func (p *objectPoolImpl) List() []Object {
	var objects []Object
	for _, obj := range p.idToObject {
		objects = append(objects, obj)
	}
	return objects
}

func (p *objectPoolImpl) Get(id string) (Object, error) {
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

func (p *objectPoolImpl) GetObjectsByInterface(interfaceType reflect.Type) ([]Object, error) {
	if interfaceType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("unsupported interface type <%s>", interfaceType.Kind())
	}

	interfaceID := generateFullType(interfaceType)
	if _, ok := p.interfaceIDToObjectTypeToImpl[interfaceID]; !ok {
		p.interfaceIDToObjectTypeToImpl[interfaceID] = make(map[string]bool)
	}

	var implObjects []Object
	for _, obj := range p.idToObject {
		impl, ok := p.interfaceIDToObjectTypeToImpl[interfaceID][obj.TypeName()]
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

		if obj.Ref() == nil {
			return nil, fmt.Errorf("object <%s> ref is nil", obj.ObjectID())
		}
		if !reflect.TypeOf(obj.Ref()).Implements(interfaceType) {
			p.interfaceIDToObjectTypeToImpl[interfaceID][obj.TypeName()] = false
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
		p.interfaceIDToObjectTypeToImpl[interfaceID][obj.TypeName()] = true
	}

	return implObjects, nil
}

func (p *objectPoolImpl) checkObjectCondition(obj Object) (bool, error) {
	if obj.Condition() == "" {
		return true, nil
	}
	return p.conditionExecutor.Execute(obj.Condition())
}

type ObjectRef interface {
	Ref() any
	Type() reflect.Type
	TypeName() string
	ObjectID() string
}

type objectRefImpl struct {
	ref      any
	t        reflect.Type
	fullType string
	objectID string
}

func newObjectRef(ref any, t reflect.Type, fullType string, objectID string) *objectRefImpl {
	return &objectRefImpl{ref: ref, t: t, fullType: fullType, objectID: objectID}
}

func (o *objectRefImpl) Ref() any {
	return o.ref
}

func (o *objectRefImpl) Type() reflect.Type {
	return o.t
}

func (o *objectRefImpl) TypeName() string {
	return o.fullType
}

func (o *objectRefImpl) ObjectID() string {
	return o.objectID
}

func parseObjectRef(ref any, name string) (ObjectRef, error) {
	rt := reflect.TypeOf(ref)
	if rt.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("unsupported object ref type <%s>, must be pointer", rt.Kind())
	}
	rt = rt.Elem()
	fullType := generateFullType(rt)
	objectID := generateObjectID(rt, name)

	return newObjectRef(ref, rt, fullType, objectID), nil
}

func generateObjectID(t reflect.Type, name string) string {
	return fmt.Sprintf("%s:%s", generateFullType(t), name)
}

func generateFullType(t reflect.Type) string {
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}
