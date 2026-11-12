package ioc

import (
	"reflect"
)

type Container interface {
	Register(rtp reflect.Type, opts ...RegisterOption) error
	GetObject(nameExpr string, rtp reflect.Type) (any, error)
	GetObjectList(nameExpr string, rtp reflect.Type) ([]any, error)
	GetObjectMap(nameExpr string, rtp reflect.Type) (map[string]any, error)
	AddValueProvider(provider ValueProvider)
	GetValue(keyExpr string, rtp reflect.Type) (any, bool, error)
}

type ContainerImpl struct {
	objectBuilderFactory ObjectBuilderFactory
	objectManager        ObjectManager
	valueManager         ValueManager
	// mu is reentrant so that Init() of an object may call back into the container on the same goroutine.
	mu reentrantMutex
}

func NewContainerImpl() *ContainerImpl {
	valueManager := newValueManagerImpl()
	conditionExecutor := newConditionExecutorImpl(valueManager)
	objectManager := newObjectManagerImpl(conditionExecutor)
	return &ContainerImpl{
		objectBuilderFactory: newObjectBuilderFactoryImpl(),
		objectManager:        objectManager,
		valueManager:         valueManager,
	}
}

func (c *ContainerImpl) Register(rtp reflect.Type, opts ...RegisterOption) error {
	if rtp.Kind() != reflect.Struct {
		return newUnsupportedRegisterType(rtp)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	ob := c.objectBuilderFactory.GetBuilder(options)
	object, err := ob.Build(rtp, options)
	if err != nil {
		return err
	}

	err = c.objectManager.AddObject(object)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) GetObject(nameExpr string, rtp reflect.Type) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	objRef, err := parseObjectRef(rtp)
	if err != nil {
		return nil, err
	}

	object, err := c.objectManager.GetObject(objRef.RType(), nameExpr)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, newMissingObjectError(objRef.FullType(), nameExpr)
	}
	if err = c.ensureInitialized(object); err != nil {
		return nil, err
	}

	return object.Instance(), nil
}

func (c *ContainerImpl) GetObjectList(nameExpr string, rtp reflect.Type) ([]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	objRef, err := parseObjectRef(rtp)
	if err != nil {
		return nil, err
	}

	objects, err := c.objectManager.GetObjects(objRef.RType(), nameExpr)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, newMissingObjectError(objRef.FullType(), nameExpr)
	}

	result := make([]any, 0, len(objects))
	for _, object := range objects {
		if err = c.ensureInitialized(object); err != nil {
			return nil, err
		}
		result = append(result, object.Instance())
	}

	return result, nil
}

func (c *ContainerImpl) GetObjectMap(nameExpr string, rtp reflect.Type) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	objRef, err := parseObjectRef(rtp)
	if err != nil {
		return nil, err
	}

	objects, err := c.objectManager.GetObjects(objRef.RType(), nameExpr)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, newMissingObjectError(objRef.FullType(), nameExpr)
	}

	nameToObject := make(map[string]any, len(objects))
	for _, object := range objects {
		if _, exist := nameToObject[object.Name()]; exist {
			return nil, newDuplicateNameObjectError(objRef.RType(), nameExpr)
		}
		if err = c.ensureInitialized(object); err != nil {
			return nil, err
		}
		nameToObject[object.Name()] = object.Instance()
	}

	return nameToObject, nil
}

// AddValueProvider and GetValue only touch the value manager, which has its own lock.
// They don't take the container lock, so they are safe to call from Init() on any goroutine.
func (c *ContainerImpl) AddValueProvider(provider ValueProvider) {
	c.valueManager.AddValueProvider(provider)
}

func (c *ContainerImpl) GetValue(keyExpr string, rtp reflect.Type) (any, bool, error) {
	value, ok, err := c.valueManager.GetValueWithType(keyExpr, rtp)
	if err != nil {
		return nil, false, err
	}

	return value, ok, nil
}

func (c *ContainerImpl) load() error {
	ctx := NewContext(nil, nil)

	requiredObjects, err := c.objectManager.ListObjects()
	if err != nil {
		return err
	}

	for _, object := range requiredObjects {
		// Initializing objects are being built further up the stack (reentrant call from Init()).
		if object.Status() != ObjectStatusDefault {
			continue
		}
		if object.Optional() {
			continue
		}
		err = c.initObject(ctx, object)
		if err != nil {
			return err
		}
	}

	return nil
}

// ensureInitialized initializes an object fetched directly, e.g. an optional object skipped by load().
func (c *ContainerImpl) ensureInitialized(object Object) error {
	if object.Status() == ObjectStatusInitialized {
		return nil
	}
	return c.initObject(NewContext(nil, nil), object)
}

func (c *ContainerImpl) initObject(ctx *InjectContext, object Object) (err error) {
	ctx = NewContext(ctx, object)
	if object.Status() == ObjectStatusInitializing {
		return newCircularDependencyError(ctx)
	}
	object.StartInitialization()
	defer func() {
		if err != nil {
			object.ResetInitialization()
		}
	}()

	var args []any

	for _, dependency := range object.Dependencies() {
		var arg any
		var err error
		switch dependency.(type) {
		case *objectDependency:
			arg, err = c.getDependencyObjectInstance(ctx, dependency.(*objectDependency))
		case *objectListDependency:
			arg, err = c.getDependencyObjectInstanceList(ctx, dependency.(*objectListDependency))
		case *objectMapDependency:
			arg, err = c.getDependencyObjectInstanceMap(ctx, dependency.(*objectMapDependency))
		case *valueDependency:
			arg, err = c.getDependencyValue(dependency.(*valueDependency))
		default:
			return newUnsupportedDependencyType(dependency)
		}
		if err != nil {
			return err
		}
		args = append(args, arg)
	}

	if err := object.Build(args); err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) getDependencyObjectInstance(ctx *InjectContext, dep *objectDependency) (any, error) {
	dependencyObject, err := c.objectManager.GetObject(dep.RType(), dep.NameExpr())
	if err != nil {
		return nil, err
	}
	if dependencyObject == nil && dep.anyName {
		dependencyObject, err = c.objectManager.GetObject(dep.RType(), "*")
		if err != nil {
			return nil, err
		}
	}

	if dependencyObject == nil {
		if !dep.Optional() {
			return nil, getMissingDependencyError(dep)
		}
		return nil, nil
	}

	if dependencyObject.Status() != ObjectStatusInitialized {
		err = c.initObject(ctx, dependencyObject)
		if err != nil {
			return nil, err
		}
	}

	return dependencyObject.Instance(), nil
}

func (c *ContainerImpl) getDependencyObjectInstanceList(ctx *InjectContext, dep *objectListDependency) (any, error) {
	implObjects, err := c.objectManager.GetObjects(dep.RType(), dep.NameExpr())
	if err != nil {
		return nil, err
	}

	if len(implObjects) == 0 {
		if !dep.Optional() {
			return nil, getMissingDependencyError(dep)
		}
		return nil, nil
	}

	objectList := reflect.MakeSlice(dep.SliceType(), 0, len(implObjects))
	for _, implObject := range implObjects {
		if implObject.Status() != ObjectStatusInitialized {
			err = c.initObject(ctx, implObject)
			if err != nil {
				return nil, err
			}
		}
		objectList = reflect.Append(objectList, reflect.ValueOf(implObject.Instance()))
	}

	return objectList.Interface(), nil
}

func (c *ContainerImpl) getDependencyObjectInstanceMap(ctx *InjectContext, dep *objectMapDependency) (any, error) {
	implObjects, err := c.objectManager.GetObjects(dep.RType(), dep.NameExpr())
	if err != nil {
		return nil, err
	}

	if len(implObjects) == 0 {
		if !dep.Optional() {
			return nil, getMissingDependencyError(dep)
		}
		return nil, nil
	}

	objectMap := reflect.MakeMapWithSize(dep.MapType(), len(implObjects))
	for _, implObject := range implObjects {
		if implObject.Status() != ObjectStatusInitialized {
			err = c.initObject(ctx, implObject)
			if err != nil {
				return nil, err
			}
		}
		key := reflect.ValueOf(implObject.Name())
		value := reflect.ValueOf(implObject.Instance())
		objectMap.SetMapIndex(key, value)
	}

	return objectMap.Interface(), nil
}

func (c *ContainerImpl) getDependencyValue(dep Dependency) (any, error) {
	value, ok, err := c.valueManager.GetValueWithType(dep.NameExpr(), dep.RType())
	if err != nil {
		return nil, err
	}

	if !ok && !dep.Optional() {
		return nil, newMissingValueError(dep.NameExpr())
	}

	return value, nil
}

func getMissingDependencyError(dep Dependency) error {
	if dep.isInterface() {
		return newMissingImplementationError(dep.FullType(), dep.NameExpr())
	}
	return newMissingObjectError(dep.FullType(), dep.NameExpr())
}
