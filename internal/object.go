package ioc

import (
	"context"
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

func (s ObjectStatus) String() string {
	switch s {
	case ObjectStatusInitializing:
		return "initializing"
	case ObjectStatusInitialized:
		return "initialized"
	default:
		return "not initialized"
	}
}

type Object interface {
	ObjectRef
	Name() string
	Aliases() []string
	Condition() string
	// Enabled reports whether the condition of the object evaluated to true on Start.
	Enabled() bool
	SetEnabled(enabled bool)
	Optional() bool
	Primary() bool
	// Provided reports whether the object is an instance passed to Provide.
	// The container doesn't call Init/Destroy on provided objects.
	Provided() bool
	Dependencies() []Dependency
	Instance() any
	Status() ObjectStatus
	StartInitialization()
	ResetInitialization()
	// Build creates the instance with the resolved dependencies.
	Build(args []any) error
	// Init runs the init hook of the built instance and marks the object as initialized.
	Init(ctx context.Context) error
	Destroy(ctx context.Context) error
}

type objectImpl struct {
	ObjectRef
	options         RegisterOptions
	provided        bool
	enabled         bool
	dependencies    []Dependency
	instanceBuilder InstanceBuilder
	instance        any
	status          ObjectStatus
}

func newObject(
	of ObjectRef,
	options RegisterOptions,
	provided bool,
	dependencies []Dependency,
	instanceBuilder InstanceBuilder,
) *objectImpl {
	return &objectImpl{
		ObjectRef:       of,
		options:         options,
		provided:        provided,
		enabled:         options.ConditionExpr == "",
		dependencies:    dependencies,
		instanceBuilder: instanceBuilder,
	}
}

func (o *objectImpl) Name() string               { return o.options.Name }
func (o *objectImpl) Aliases() []string          { return o.options.Aliases }
func (o *objectImpl) Condition() string          { return o.options.ConditionExpr }
func (o *objectImpl) Enabled() bool              { return o.enabled }
func (o *objectImpl) SetEnabled(enabled bool)    { o.enabled = enabled }
func (o *objectImpl) Optional() bool             { return o.options.Optional }
func (o *objectImpl) Primary() bool              { return o.options.Primary }
func (o *objectImpl) Provided() bool             { return o.provided }
func (o *objectImpl) Dependencies() []Dependency { return o.dependencies }
func (o *objectImpl) Instance() any              { return o.instance }
func (o *objectImpl) Status() ObjectStatus       { return o.status }

func (o *objectImpl) StartInitialization() {
	o.status = ObjectStatusInitializing
}

// ResetInitialization rolls the object back to the default status after a failed initialization,
// so that a later retry is not mistaken for a circular dependency.
func (o *objectImpl) ResetInitialization() {
	o.status = ObjectStatusDefault
	o.instance = nil
}

func (o *objectImpl) Build(args []any) error {
	instance, err := o.instanceBuilder.Build(args)
	if err != nil {
		return err
	}
	o.instance = instance
	return nil
}

func (o *objectImpl) Init(ctx context.Context) error {
	if !o.provided {
		if err := initInstance(ctx, o.instance); err != nil {
			return err
		}
	}
	o.status = ObjectStatusInitialized
	return nil
}

func (o *objectImpl) Destroy(ctx context.Context) error {
	if o.provided || o.status != ObjectStatusInitialized {
		return nil
	}
	return destroyInstance(ctx, o.instance)
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

type ObjectBuilder interface {
	Build(rtp reflect.Type, options RegisterOptions) (Object, error)
}

type baseObjectBuilder struct{}

// parseDependencies collects the `inject` and `value` tagged fields of rtp, recursing into untagged struct fields.
// fIndex is the index path of rtp, and location is its description used in error messages.
func (b *baseObjectBuilder) parseDependencies(rtp reflect.Type, fIndex []int, location string) ([]Dependency, [][]int, error) {
	var injectFieldIndexes [][]int
	var dependencies []Dependency

	var fn func(rtp reflect.Type, fIndex []int, path string) error
	fn = func(rtp reflect.Type, fIndex []int, path string) error {
		for i := 0; i < rtp.NumField(); i++ {
			field := rtp.Field(i)
			// copy to avoid sharing the backing array between sibling fields
			fi := make([]int, len(fIndex)+1)
			copy(fi, fIndex)
			fi[len(fIndex)] = i
			fieldPath := field.Name
			if path != "" {
				fieldPath = path + "." + field.Name
			}
			fieldLocation := "field " + fieldPath
			if location != "" {
				fieldLocation = location + " " + fieldLocation
			}

			var dependency Dependency
			if injectTagExpr, ok := field.Tag.Lookup(TagInjectKey); ok {
				var err error
				dependency, err = b.parseInjectDependency(field, ParseInjectTag(injectTagExpr), fieldLocation)
				if err != nil {
					return err
				}
			} else if valueTagExpr, ok := field.Tag.Lookup(TagValueKey); ok {
				valueTag := ParseValueTag(valueTagExpr)
				if err := validateValueType(field.Type, valueTag); err != nil {
					return fmt.Errorf("%s: %w", fieldLocation, err)
				}
				dependency = newValueDependency(valueTag, field.Type, fieldLocation)
			} else if field.Type.Kind() == reflect.Struct {
				if err := fn(field.Type, fi, fieldPath); err != nil {
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

	if err := fn(rtp, fIndex, ""); err != nil {
		return nil, nil, err
	}

	return dependencies, injectFieldIndexes, nil
}

func (b *baseObjectBuilder) parseInjectDependency(field reflect.StructField, injectTag InjectTag, location string) (Dependency, error) {
	ft := field.Type
	switch ft.Kind() {
	case reflect.Ptr:
		if ft.Elem().Kind() == reflect.Struct {
			// eg. "inject:*MyStruct"
			return newObjectDependency(injectTag.Value(), ft.Elem(), injectTag.Optional(), location), nil
		}
	case reflect.Interface:
		// eg. "inject:MyInterface"
		return newObjectDependency(injectTag.Value(), ft, injectTag.Optional(), location), nil
	case reflect.Slice:
		if ft.Elem().Kind() == reflect.Interface {
			// eg. "inject:[]MyInterface"
			return newObjectListDependency(injectTag.Value(), ft.Elem(), injectTag.Optional(), location), nil
		} else if ft.Elem().Kind() == reflect.Ptr && ft.Elem().Elem().Kind() == reflect.Struct {
			// eg. "inject:[]*MyStruct"
			return newObjectListDependency(injectTag.Value(), ft.Elem().Elem(), injectTag.Optional(), location), nil
		}
	case reflect.Map:
		if ft.Key().Kind() != reflect.String {
			break
		}
		if ft.Elem().Kind() == reflect.Ptr && ft.Elem().Elem().Kind() == reflect.Struct {
			// eg. "inject:map[string]*MyStruct"
			return newObjectMapDependency(injectTag.Value(), ft.Elem().Elem(), injectTag.Optional(), location), nil
		} else if ft.Elem().Kind() == reflect.Interface {
			// eg. "inject:map[string]MyInterface"
			return newObjectMapDependency(injectTag.Value(), ft.Elem(), injectTag.Optional(), location), nil
		}
	}
	// eg. "inject:MyStruct", "inject:*int", "inject:[]int", "inject:map[int]MyInterface"
	return nil, newUnsupportedInjectFieldTypeError(field)
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

	dependencies, injectFieldIndexes, err := f.parseDependencies(objRef.RType(), nil, "")
	if err != nil {
		return nil, err
	}

	return newObject(objRef, options, false, dependencies, newFieldInstanceBuilder(objRef.RType(), injectFieldIndexes)), nil
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
	if ct.NumOut() > 2 || ct.NumOut() == 0 || (ct.NumOut() == 2 && ct.Out(1) != errorType) {
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
		location := fmt.Sprintf("constructor param #%d", i)
		var dependency Dependency
		switch pt.Kind() {
		case reflect.Ptr:
			if pt.Elem().Kind() != reflect.Struct {
				return nil, newUnsupportedConstructorParamTypeError(options.Constructor, pt)
			}
			dependency = newObjectDependency("", pt.Elem(), false, location)
		case reflect.Interface:
			dependency = newObjectDependency("", pt, false, location)
		case reflect.Struct:
			deps, indexes, err := c.parseDependencies(pt, ai, location)
			if err != nil {
				return nil, err
			}
			dependencies = append(dependencies, deps...)
			injectArgIndexes = append(injectArgIndexes, indexes...)
			continue
		default:
			return nil, newUnsupportedConstructorParamTypeError(options.Constructor, pt)
		}
		dependencies = append(dependencies, dependency)
		injectArgIndexes = append(injectArgIndexes, ai)
	}

	return newObject(of, options, false, dependencies, newConstructorInstanceBuilder(options.Constructor, injectArgIndexes)), nil
}

// newProvidedObject creates an object from an existing instance.
// A pointer to struct is registered as the struct type; other types (e.g. func types) are registered as is
// and can only be injected through the interfaces they implement.
func newProvidedObject(instance any, options RegisterOptions) (Object, error) {
	if options.Constructor != nil {
		return nil, newInvalidProvideError(instance, "constructor option is not allowed")
	}
	if instance == nil {
		return nil, newInvalidProvideError(instance, "instance is nil")
	}

	rv := reflect.ValueOf(instance)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Func, reflect.Chan, reflect.Slice, reflect.Interface:
		if rv.IsNil() {
			return nil, newInvalidProvideError(instance, "instance is nil")
		}
	case reflect.Struct:
		return nil, newInvalidProvideError(instance, "struct value is not allowed, pass a pointer instead")
	}

	rtp := rv.Type()
	if rtp.Kind() == reflect.Ptr && rtp.Elem().Kind() == reflect.Struct {
		rtp = rtp.Elem()
	}

	return newObject(newObjectRef(rtp), options, true, nil, newProvidedInstanceBuilder(instance)), nil
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
	// AllObjects returns all registered objects in registration order.
	AllObjects() []Object
	// EvaluateConditions evaluates the condition of every object and stores the result.
	EvaluateConditions() error
	// GetObject returns the single object matching rtp and nameExpr, or nil if none.
	// An empty nameExpr matches by type: the only unnamed object, then the only primary object,
	// then the only object of that type.
	GetObject(rtp reflect.Type, nameExpr string, initializedOnly bool) (Object, error)
	GetObjects(rtp reflect.Type, nameExpr string, initializedOnly bool) ([]Object, error)
}

type objectManagerImpl struct {
	mu      sync.Mutex
	objects []Object
	// keyed by reflect.Type rather than type name: anonymous interfaces and
	// function-local types may share the same name string.
	registeredTypeToName map[reflect.Type]map[string]bool
	// interface type -> object type -> whether the object implements the interface
	implementsCache   map[reflect.Type]map[reflect.Type]bool
	conditionExecutor ConditionExecutor
}

func newObjectManagerImpl(conditionExecutor ConditionExecutor) *objectManagerImpl {
	return &objectManagerImpl{
		registeredTypeToName: make(map[reflect.Type]map[string]bool),
		implementsCache:      make(map[reflect.Type]map[reflect.Type]bool),
		conditionExecutor:    conditionExecutor,
	}
}

func (p *objectManagerImpl) AddObject(object Object) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	names, ok := p.registeredTypeToName[object.RType()]
	if !ok {
		names = make(map[string]bool)
		p.registeredTypeToName[object.RType()] = names
	}
	if names[object.Name()] {
		return newObjectDuplicateRegisterError(object.FullType(), object.Name())
	}
	names[object.Name()] = true
	p.objects = append(p.objects, object)
	return nil
}

func (p *objectManagerImpl) AllObjects() []Object {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]Object(nil), p.objects...)
}

func (p *objectManagerImpl) EvaluateConditions() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, obj := range p.objects {
		if obj.Condition() == "" {
			obj.SetEnabled(true)
			continue
		}
		enabled, err := p.conditionExecutor.Execute(obj.Condition())
		if err != nil {
			return fmt.Errorf("evaluate condition of <%s>: %w", generateFullName(obj.FullType(), obj.Name()), err)
		}
		obj.SetEnabled(enabled)
	}
	return nil
}

func (p *objectManagerImpl) GetObject(rtp reflect.Type, nameExpr string, initializedOnly bool) (Object, error) {
	objects, err := p.GetObjects(rtp, nameExpr, initializedOnly)
	if err != nil {
		return nil, err
	}

	switch len(objects) {
	case 0:
		return nil, nil
	case 1:
		return objects[0], nil
	}

	if nameExpr == "" {
		if obj := pickDefaultObject(objects); obj != nil {
			return obj, nil
		}
	}

	if rtp.Kind() == reflect.Interface {
		return nil, newMultipleImplementationError(rtp, nameExpr, objects)
	}
	return nil, newMultipleObjectError(rtp, nameExpr, objects)
}

// pickDefaultObject picks the only unnamed object, or else the only primary object.
func pickDefaultObject(objects []Object) Object {
	var unnamed, primary []Object
	for _, obj := range objects {
		if obj.Name() == "" {
			unnamed = append(unnamed, obj)
		}
		if obj.Primary() {
			primary = append(primary, obj)
		}
	}
	if len(unnamed) == 1 {
		return unnamed[0]
	}
	if len(primary) == 1 {
		return primary[0]
	}
	return nil
}

// GetObjects returns the enabled objects matching rtp and nameExpr.
// An empty nameExpr matches objects of any name.
func (p *objectManagerImpl) GetObjects(rtp reflect.Type, nameExpr string, initializedOnly bool) ([]Object, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if rtp.Kind() != reflect.Struct && rtp.Kind() != reflect.Interface {
		return nil, newUnsupportedObjectTypeError(rtp)
	}

	var objects []Object
	for _, object := range p.objects {
		if !object.Enabled() {
			continue
		}
		if initializedOnly && object.Status() != ObjectStatusInitialized {
			continue
		}
		if rtp.Kind() == reflect.Struct {
			if object.RType() != rtp {
				continue
			}
		} else if !p.implements(object, rtp) {
			continue
		}

		if nameExpr != "" {
			matched, err := p.matchObjectName(object, nameExpr)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
		}

		objects = append(objects, object)
	}

	return objects, nil
}

func (p *objectManagerImpl) implements(object Object, iface reflect.Type) bool {
	cache, ok := p.implementsCache[iface]
	if !ok {
		cache = make(map[reflect.Type]bool)
		p.implementsCache[iface] = cache
	}
	impl, ok := cache[object.RType()]
	if !ok {
		impl = object.Implements(iface)
		cache[object.RType()] = impl
	}
	return impl
}

func (p *objectManagerImpl) matchObjectName(obj Object, nameExpr string) (bool, error) {
	for _, name := range append([]string{obj.Name()}, obj.Aliases()...) {
		matched, err := filepath.Match(nameExpr, name)
		if err != nil {
			return false, fmt.Errorf("invalid name expression %q: %w", nameExpr, err)
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

func newObjectRef(t reflect.Type) *objectRefImpl {
	return &objectRefImpl{rtp: t, fullType: generateFullType(t)}
}

func (o *objectRefImpl) RType() reflect.Type {
	return o.rtp
}

func (o *objectRefImpl) FullType() string {
	return o.fullType
}

// Implements reports whether the instance of the object implements rtp.
// Instances of struct types are stored as pointers.
func (o *objectRefImpl) Implements(rtp reflect.Type) bool {
	if o.rtp.Kind() == reflect.Struct {
		return reflect.PointerTo(o.rtp).Implements(rtp)
	}
	return o.rtp.Implements(rtp)
}

func parseObjectRef(rtp reflect.Type) (ObjectRef, error) {
	if rtp.Kind() != reflect.Struct && rtp.Kind() != reflect.Interface {
		return nil, newUnsupportedObjectRefTypeError(rtp)
	}
	return newObjectRef(rtp), nil
}

func generateFullType(t reflect.Type) string {
	if t.Name() == "" {
		// unnamed types, e.g. `interface{ Foo() }` or `func()`
		return t.String()
	}
	if t.PkgPath() == "" {
		return t.Name()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

func generateFullName(fullType, name string) string {
	return fmt.Sprintf(`%s@"%s"`, fullType, name)
}
