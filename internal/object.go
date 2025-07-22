package ioc

import (
	"fmt"
	"path/filepath"
	"reflect"
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
	Name() string
	Aliases() []string
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
	aliases         []string
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
	aliases []string,
	condition string,
	optional bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
) *objectImpl {
	return &objectImpl{
		ObjectRef:       of,
		name:            name,
		aliases:         aliases,
		condition:       condition,
		optional:        optional,
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
	}
}

func (o *objectImpl) Name() string {
	return o.name
}

func (o *objectImpl) Aliases() []string {
	return o.aliases
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
	Build(rtp reflect.Type, options RegisterOptions) (Object, error)
}

type baseObjectBuilder struct{}

func (b *baseObjectBuilder) parseDependencies(rtp reflect.Type, fIndex []int) ([]Dependency, [][]int, error) {
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
				var err error
				dependency, err = b.parseInjectDependency(field, injectTag)
				if err != nil {
					return err
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

func (b *baseObjectBuilder) parseInjectDependency(field reflect.StructField, injectTag InjectTag) (Dependency, error) {
	switch field.Type.Kind() {
	case reflect.Ptr:
		if field.Type.Elem().Kind() == reflect.Struct {
			// eg. "inject:*MyStruct"
			return newObjectDependency(injectTag.Value(), field.Type.Elem(), injectTag.Optional()), nil
		} else {
			// eg. "inject:*MyInterface or "inject:*int"
			return nil, newUnsupportedInjectFieldTypeError(field)
		}
	case reflect.Interface:
		// eg. "inject:MyInterface"
		return newObjectDependency(injectTag.Value(), field.Type, injectTag.Optional()), nil
	case reflect.Slice:
		if field.Type.Elem().Kind() == reflect.Interface {
			// eg. "inject:[]MyInterface"
			return newObjectListDependency(injectTag.Value(), field.Type.Elem(), injectTag.Optional()), nil
		} else if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
			// eg. "inject:[]*MyStruct"
			return newObjectListDependency(injectTag.Value(), field.Type.Elem().Elem(), injectTag.Optional()), nil
		} else {
			// eg. "inject:[]int"
			return nil, newUnsupportedInjectFieldTypeError(field)
		}
	case reflect.Map:
		if field.Type.Key().Kind() != reflect.String {
			// eg. "inject:map[int]string"
			return nil, newUnsupportedInjectFieldTypeError(field)
		}
		if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
			// eg. "inject:map[string]*MyStruct"
			return newObjectMapDependency(injectTag.Value(), field.Type.Elem().Elem(), injectTag.Optional()), nil
		} else if field.Type.Elem().Kind() == reflect.Interface {
			// eg. "inject:map[string]MyInterface"
			return newObjectMapDependency(injectTag.Value(), field.Type.Elem(), injectTag.Optional()), nil
		} else {
			// eg. "inject:map[string]int"
			return nil, newUnsupportedInjectFieldTypeError(field)
		}
	default:
		// eg. "inject:MyStruct or "inject:int"
		return nil, newUnsupportedInjectFieldTypeError(field)
	}
}

type fieldsObjectBuilder struct {
	*baseObjectBuilder
}

func newFieldsObjectBuilder() *fieldsObjectBuilder {
	return &fieldsObjectBuilder{}
}

func (f *fieldsObjectBuilder) Build(rtp reflect.Type, options RegisterOptions) (Object, error) {
	objRef, err := parseObjectRef(rtp)
	if err != nil {
		return nil, err
	}

	dependencies, injectFieldIndexes, err := f.parseDependencies(objRef.RType(), nil)
	if err != nil {
		return nil, err
	}

	obj := newObject(
		objRef,
		options.Name,
		options.Aliases,
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

func (c *constructorObjectBuilder) Build(rtp reflect.Type, options RegisterOptions) (Object, error) {
	of, err := parseObjectRef(rtp)
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
	if ct.Out(0).Kind() != reflect.Ptr || rtp != ct.Out(0).Elem() {
		return nil, newConstructorNotReturnObjectError(options.Constructor, rtp)
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
		options.Name,
		options.Aliases,
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

type ObjectManager interface {
	AddObject(object Object) error
	ListObjects() ([]Object, error)
	GetObject(rtp reflect.Type, nameExpr string) (Object, error)
	GetObjects(rtp reflect.Type, nameExpr string) ([]Object, error)
}

type objectManagerImpl struct {
	mu                          sync.Mutex
	objects                     []Object
	registeredObjectTypeToName  map[string]map[string]bool
	interfaceToObjectTypeToImpl map[string]map[string]bool
	conditionExecutor           ConditionExecutor
}

func newObjectManagerImpl(conditionExecutor ConditionExecutor) *objectManagerImpl {
	return &objectManagerImpl{
		conditionExecutor: conditionExecutor,
	}
}

func (p *objectManagerImpl) AddObject(object Object) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.registeredObjectTypeToName[object.FullType()][object.Name()] {
		return newObjectDuplicateRegisterError(object.FullType(), object.Name())
	}
	if p.registeredObjectTypeToName == nil {
		p.registeredObjectTypeToName = make(map[string]map[string]bool)
	}
	if _, ok := p.registeredObjectTypeToName[object.FullType()]; !ok {
		p.registeredObjectTypeToName[object.FullType()] = make(map[string]bool)
	}
	p.registeredObjectTypeToName[object.FullType()][object.Name()] = true
	p.objects = append(p.objects, object)
	return nil
}

func (p *objectManagerImpl) ListObjects() ([]Object, error) {
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

func (p *objectManagerImpl) GetObject(rtp reflect.Type, nameExpr string) (Object, error) {
	objects, err := p.GetObjects(rtp, nameExpr)
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, nil
	}
	if len(objects) > 1 {
		if rtp.Kind() == reflect.Interface {
			return nil, newMultipleImplementationError(rtp, nameExpr, objects)
		}
		return nil, newMultipleObjectError(rtp, nameExpr, objects)
	}

	return objects[0], nil
}

func (p *objectManagerImpl) GetObjects(rtp reflect.Type, nameExpr string) ([]Object, error) {
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

func (p *objectManagerImpl) checkObjectCondition(obj Object) (bool, error) {
	if obj.Condition() == "" {
		return true, nil
	}
	return p.conditionExecutor.Execute(obj.Condition())
}

func (p *objectManagerImpl) checkObjectName(obj Object, nameExpr string) (bool, error) {
	for _, name := range append([]string{obj.Name()}, obj.Aliases()...) {
		matched, err := filepath.Match(nameExpr, name)
		if err != nil {
			return false, fmt.Errorf("match object name expr failed, err=%w", err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

type ObjectRef interface {
	RType() reflect.Type
	FullType() string
	Implements(rtp reflect.Type) bool
}

type objectRefImpl struct {
	rtp      reflect.Type
	fullType string
}

func newObjectRef(t reflect.Type, fullType string) *objectRefImpl {
	return &objectRefImpl{rtp: t, fullType: fullType}
}

func (o *objectRefImpl) RType() reflect.Type {
	return o.rtp
}

func (o *objectRefImpl) FullType() string {
	return o.fullType
}

func (o *objectRefImpl) Implements(rtp reflect.Type) bool {
	return o.rtp.Implements(rtp) || reflect.New(o.rtp).Type().Implements(rtp)
}

func parseObjectRef(rtp reflect.Type) (ObjectRef, error) {
	if rtp.Kind() != reflect.Struct && rtp.Kind() != reflect.Interface {
		return nil, newUnsupportedObjectRefTypeError(rtp)
	}
	fullType := generateFullType(rtp)

	return newObjectRef(rtp, fullType), nil
}

func generateFullType(t reflect.Type) string {
	if t.PkgPath() == "" {
		return t.Name()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

func generateFullName(fullType, name string) string {
	return fmt.Sprintf("%s@%s", fullType, name)
}
