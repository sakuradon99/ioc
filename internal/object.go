package ioc

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

type ObjectStatus int

const (
	ObjectStatusDefault ObjectStatus = iota
	ObjectStatusInitializing
	ObjectStatusInitialized
)

type Object interface {
	ObjectRef
	NameExpr() string
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
	nameExpr        string
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
	nameExpr string,
	condition string,
	optional bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
) *objectImpl {
	return &objectImpl{
		ObjectRef:       of,
		nameExpr:        nameExpr,
		condition:       condition,
		optional:        optional,
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
	}
}

func (o *objectImpl) NameExpr() string {
	return o.nameExpr
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

type baseObjectBuilder struct{}

func (*baseObjectBuilder) parseDependencies(rtp reflect.Type, fIndex []int) ([]Dependency, [][]int, error) {
	var injectFieldIndexes [][]int
	var dependencies []Dependency

	var fn func(rtp reflect.Type, fIndex []int) error
	fn = func(rtp reflect.Type, fIndex []int) error {
		for i := 0; i < rtp.NumField(); i++ {
			field := rtp.Field(i)
			fi := append(fIndex, i)
			var dependency Dependency
			if injectTagExpr, ok := field.Tag.Lookup(TagInjectKey); ok {
				injectTag := ParseInjectTag(injectTagExpr)
				switch field.Type.Kind() {
				case reflect.Ptr:
					if field.Type.Elem().Kind() == reflect.Struct {
						dependency = newObjectDependency(injectTag.Value(), field.Type.Elem(), injectTag.Optional())
					} else {
						return newUnsupportedInjectFieldTypeError(field)
					}
				case reflect.Interface:
					dependency = newObjectDependency(injectTag.Value(), field.Type, injectTag.Optional())
				case reflect.Slice:
					if field.Type.Elem().Kind() == reflect.Interface {
						dependency = newObjectsDependency(injectTag.Value(), field.Type.Elem(), injectTag.Optional())
					} else if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
						dependency = newObjectsDependency(injectTag.Value(), field.Type.Elem().Elem(), injectTag.Optional())
					} else {
						return newUnsupportedInjectFieldTypeError(field)
					}
				default:
					return newUnsupportedInjectFieldTypeError(field)
				}
			} else if valueTagExpr, ok := field.Tag.Lookup(TagValueKey); ok {
				valueTag := ParseValueTag(valueTagExpr)
				dependency = newValueDependency(valueTag.Value(), field.Type, valueTag.Optional())
			} else if field.Type.Kind() == reflect.Struct {
				err := fn(field.Type, fi)
				if err != nil {
					return err
				}
			}
			if dependency != nil {
				injectFieldIndexes = append(injectFieldIndexes, fi)
				dependencies = append(dependencies, dependency)
			}
		}
		return nil
	}

	err := fn(rtp, fIndex)
	if err != nil {
		return nil, nil, err
	}

	return dependencies, injectFieldIndexes, nil
}

type fieldsObjectBuilder struct {
	*baseObjectBuilder
}

func newFieldsObjectBuilder() *fieldsObjectBuilder {
	return &fieldsObjectBuilder{}
}

func (f *fieldsObjectBuilder) Build(ref any, options RegisterOptions) (Object, error) {
	objRef, err := parseObjectRef(ref)
	if err != nil {
		return nil, err
	}

	dependencies, injectFieldIndexes, err := f.parseDependencies(objRef.RType(), nil)
	if err != nil {
		return nil, err
	}

	obj := newObject(
		objRef,
		options.NameExpr,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		newFieldInstanceBuilder(objRef.RType(), injectFieldIndexes),
	)

	return obj, nil
}

type constructorObjectBuilder struct {
	*baseObjectBuilder
}

func newConstructorObjectBuilder() *constructorObjectBuilder {
	return &constructorObjectBuilder{}
}

func (c *constructorObjectBuilder) Build(ref any, options RegisterOptions) (Object, error) {
	of, err := parseObjectRef(ref)
	if err != nil {
		return nil, err
	}

	ct := reflect.TypeOf(options.Constructor)
	if ct.Kind() != reflect.Func {
		return nil, newUnsupportedConstructorError(options.Constructor)
	}
	if ct.NumOut() > 2 || ct.NumOut() == 0 || (ct.NumOut() == 2 && ct.Out(1).Name() != "error") {
		return nil, newUnsupportedConstructorError(options.Constructor)
	}
	if reflect.TypeOf(ref) != ct.Out(0) {
		return nil, newConstructorNotReturnObjectError(options.Constructor, reflect.TypeOf(ref))
	}

	var dependencies []Dependency
	var injectArgIndexes [][]int

	for i := 0; i < ct.NumIn(); i++ {
		pt := ct.In(i)
		ai := []int{i}
		var dependency Dependency
		switch pt.Kind() {
		case reflect.Ptr:
			dependency = newObjectDependency("", pt.Elem(), true)
		case reflect.Interface:
			dependency = newObjectDependency("", pt, true)
		case reflect.Struct:
			deps, indexes, err := c.parseDependencies(pt, ai)
			if err != nil {
				return nil, err
			}
			dependencies = append(dependencies, deps...)
			injectArgIndexes = append(injectArgIndexes, indexes...)
			continue
		default:
			return nil, newUnsupportedConstructorParamTypeError(options.Constructor, pt)
		}
		if dependency != nil {
			dependencies = append(dependencies, dependency)
			injectArgIndexes = append(injectArgIndexes, ai)
		}
	}

	obj := newObject(
		of,
		options.NameExpr,
		options.ConditionExpr,
		options.Optional,
		dependencies,
		newConstructorInstanceBuilder(options.Constructor, injectArgIndexes),
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
	AddObject(object Object) error
	ListObjects() ([]Object, error)
	GetObject(rtp reflect.Type, nameExpr string) (Object, error)
	GetObjects(rtp reflect.Type, nameExpr string) ([]Object, error)
}

type objectPoolImpl struct {
	mu                          sync.Mutex
	objects                     []Object
	registeredObjectTypeToName  map[string]map[string]bool
	interfaceToObjectTypeToImpl map[string]map[string]bool
	conditionExecutor           ConditionExecutor
}

func newObjectPoolImpl(conditionExecutor ConditionExecutor) *objectPoolImpl {
	return &objectPoolImpl{
		conditionExecutor: conditionExecutor,
	}
}

func (p *objectPoolImpl) AddObject(object Object) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.registeredObjectTypeToName[object.FullType()][object.NameExpr()] {
		return newObjectDuplicateRegisterError(object.FullType(), object.NameExpr())
	}
	if p.registeredObjectTypeToName == nil {
		p.registeredObjectTypeToName = make(map[string]map[string]bool)
	}
	if _, ok := p.registeredObjectTypeToName[object.FullType()]; !ok {
		p.registeredObjectTypeToName[object.FullType()] = make(map[string]bool)
	}
	p.registeredObjectTypeToName[object.FullType()][object.NameExpr()] = true
	p.objects = append(p.objects, object)
	return nil
}

func (p *objectPoolImpl) ListObjects() ([]Object, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var objects []Object
	for _, obj := range p.objects {
		checked, err := p.checkObjectCondition(obj)
		if err != nil {
			return nil, err
		}
		if !checked {
			continue
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func (p *objectPoolImpl) GetObject(rtp reflect.Type, nameExpr string) (Object, error) {
	objects, err := p.GetObjects(rtp, nameExpr)
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, nil
	}
	if len(objects) > 1 {
		if rtp.Kind() == reflect.Interface {
			return nil, newMultipleImplementationError(rtp, nameExpr)
		}
		return nil, newMultipleObjectError(rtp, nameExpr)
	}

	return objects[0], nil
}

func (p *objectPoolImpl) GetObjects(rtp reflect.Type, nameExpr string) ([]Object, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var objects []Object
	fullType := generateFullType(rtp)

	for _, object := range p.objects {
		switch rtp.Kind() {
		case reflect.Struct:
			if object.RType() != rtp {
				continue
			}
		case reflect.Interface:
			if p.interfaceToObjectTypeToImpl == nil {
				p.interfaceToObjectTypeToImpl = make(map[string]map[string]bool)
			}
			if _, ok := p.interfaceToObjectTypeToImpl[fullType]; !ok {
				p.interfaceToObjectTypeToImpl[fullType] = make(map[string]bool)
			}
			if !p.interfaceToObjectTypeToImpl[fullType][object.FullType()] {
				if !object.Implements(rtp) {
					p.interfaceToObjectTypeToImpl[fullType][object.FullType()] = false
					continue
				}
				p.interfaceToObjectTypeToImpl[fullType][object.FullType()] = true
			}
		default:
			return nil, newUnsupportedObjectTypeError(rtp)
		}

		checked, err := p.checkObjectName(object, nameExpr)
		if err != nil {
			return nil, err
		}
		if !checked {
			continue
		}
		checked, err = p.checkObjectCondition(object)
		if err != nil {
			return nil, err
		}
		if !checked {
			continue
		}

		objects = append(objects, object)
	}

	return objects, nil
}

func (p *objectPoolImpl) checkObjectCondition(obj Object) (bool, error) {
	if obj.Condition() == "" {
		return true, nil
	}
	return p.conditionExecutor.Execute(obj.Condition())
}

func (p *objectPoolImpl) checkObjectName(obj Object, nameExpr string) (bool, error) {
	if strings.HasPrefix(nameExpr, "r:") {
		reg := strings.TrimPrefix(nameExpr, "r:")
		return regexp.MatchString(reg, obj.NameExpr())
	}
	return obj.NameExpr() == nameExpr, nil
}

type ObjectRef interface {
	Ref() any
	RType() reflect.Type
	FullType() string
	Implements(rtp reflect.Type) bool
}

type objectRefImpl struct {
	ref      any
	t        reflect.Type
	fullType string
}

func newObjectRef(ref any, t reflect.Type, fullType string) *objectRefImpl {
	return &objectRefImpl{ref: ref, t: t, fullType: fullType}
}

func (o *objectRefImpl) Ref() any {
	return o.ref
}

func (o *objectRefImpl) RType() reflect.Type {
	return o.t
}

func (o *objectRefImpl) FullType() string {
	return o.fullType
}

func (o *objectRefImpl) Implements(rtp reflect.Type) bool {
	return reflect.TypeOf(o.ref).Implements(rtp)
}

func parseObjectRef(ref any) (ObjectRef, error) {
	rt := reflect.TypeOf(ref)
	if rt.Kind() != reflect.Ptr {
		return nil, newObjectRefNotPointerError(rt)
	}
	rt = rt.Elem()
	if rt.Kind() != reflect.Struct && rt.Kind() != reflect.Interface {
		return nil, newUnsupportedObjectRefTypeError(rt)
	}
	fullType := generateFullType(rt)

	return newObjectRef(ref, rt, fullType), nil
}

func generateFullType(t reflect.Type) string {
	if t.PkgPath() == "" {
		return t.Name()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}
