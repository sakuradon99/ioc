package ioc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

type Container interface {
	Register(rtp reflect.Type, opts ...RegisterOption) error
	Provide(instance any, opts ...RegisterOption) error
	AddValueProvider(provider ValueProvider) error
	// Start loads the value providers and initializes all non-optional objects.
	// It must be called before getting any object or value.
	Start(ctx context.Context) error
	// Shutdown destroys the initialized objects in reverse initialization order.
	Shutdown(ctx context.Context) error
	GetObject(nameExpr string, rtp reflect.Type) (any, error)
	GetObjectList(nameExpr string, rtp reflect.Type) ([]any, error)
	GetObjectMap(nameExpr string, rtp reflect.Type) (map[string]any, error)
	GetValue(keyExpr string, rtp reflect.Type) (any, bool, error)
	// Dump describes the registered objects and their dependencies, for debugging.
	Dump() string
}

type containerState int32

const (
	stateNew containerState = iota
	stateStarting
	stateStarted
	stateShutdown
)

func (s containerState) String() string {
	switch s {
	case stateStarting:
		return "starting"
	case stateStarted:
		return "started"
	case stateShutdown:
		return "shutdown"
	default:
		return "new"
	}
}

type ContainerImpl struct {
	objectBuilderFactory ObjectBuilderFactory
	objectManager        ObjectManager
	valueManager         ValueManager

	// mu is held exclusively while mutating the container (Register, Start, Shutdown...),
	// and shared by the read-only operations after Start.
	mu    sync.RWMutex
	state atomic.Int32

	// fields below are guarded by mu
	startCtx  context.Context
	failed    map[Object]error // objects failed in the current Start, to report each failure once
	initOrder []Object
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

func (c *ContainerImpl) getState() containerState {
	return containerState(c.state.Load())
}

func (c *ContainerImpl) setState(state containerState) {
	c.state.Store(int32(state))
}

// checkMutable reports whether the container can still be configured, i.e. it is not started yet.
// It is checked before taking the lock, so that calls from Init during Start fail instead of deadlocking.
func (c *ContainerImpl) checkMutable() error {
	switch c.getState() {
	case stateStarting:
		return ErrStarting
	case stateStarted:
		return ErrAlreadyStarted
	case stateShutdown:
		return ErrShutdown
	}
	return nil
}

// checkReadable reports whether objects and values can be read, i.e. the container is started.
func (c *ContainerImpl) checkReadable() error {
	switch c.getState() {
	case stateNew:
		return ErrNotStarted
	case stateStarting:
		return ErrStarting
	case stateShutdown:
		return ErrShutdown
	}
	return nil
}

func (c *ContainerImpl) Register(rtp reflect.Type, opts ...RegisterOption) error {
	if rtp.Kind() != reflect.Struct {
		return newUnsupportedRegisterType(rtp)
	}

	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	return c.addObject(func() (Object, error) {
		return c.objectBuilderFactory.GetBuilder(options).Build(rtp, options)
	})
}

func (c *ContainerImpl) Provide(instance any, opts ...RegisterOption) error {
	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	return c.addObject(func() (Object, error) {
		return newProvidedObject(instance, options)
	})
}

func (c *ContainerImpl) addObject(build func() (Object, error)) error {
	if err := c.checkMutable(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkMutable(); err != nil {
		return err
	}

	object, err := build()
	if err != nil {
		return err
	}

	return c.objectManager.AddObject(object)
}

func (c *ContainerImpl) AddValueProvider(provider ValueProvider) error {
	if err := c.checkMutable(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkMutable(); err != nil {
		return err
	}

	c.valueManager.AddValueProvider(provider)
	return nil
}

func (c *ContainerImpl) Start(ctx context.Context) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err = c.checkMutable(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err = c.checkMutable(); err != nil {
		return err
	}

	c.setState(stateStarting)
	c.startCtx = ctx
	c.failed = make(map[Object]error)
	defer func() {
		c.startCtx = nil
		c.failed = nil
		if err != nil {
			// allow fixing the problem (e.g. registering a missing object) and starting again
			c.setState(stateNew)
		} else {
			c.setState(stateStarted)
		}
	}()

	if err = c.valueManager.Load(); err != nil {
		return fmt.Errorf("load value providers: %w", err)
	}
	if err = c.objectManager.EvaluateConditions(); err != nil {
		return err
	}

	var errs []error
	for _, object := range c.objectManager.AllObjects() {
		if !object.Enabled() || object.Optional() || object.Status() != ObjectStatusDefault {
			continue
		}
		if err = ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err = c.initObject(NewContext(nil, nil), object); err != nil {
			errs = appendUniqueError(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c *ContainerImpl) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	switch c.getState() {
	case stateStarting:
		return ErrStarting
	case stateShutdown:
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.getState() == stateShutdown {
		return nil
	}
	c.setState(stateShutdown)

	var errs []error
	for i := len(c.initOrder) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		object := c.initOrder[i]
		if err := object.Destroy(ctx); err != nil {
			errs = append(errs, fmt.Errorf("destroy <%s>: %w", generateFullName(object.FullType(), object.Name()), err))
		}
	}
	c.initOrder = nil

	return errors.Join(errs...)
}

func (c *ContainerImpl) GetObject(nameExpr string, rtp reflect.Type) (any, error) {
	objRef, err := c.beginRead(rtp)
	if err != nil {
		return nil, err
	}
	defer c.mu.RUnlock()

	object, err := c.objectManager.GetObject(objRef.RType(), nameExpr, true)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, c.notFoundError(objRef, nameExpr)
	}

	return object.Instance(), nil
}

func (c *ContainerImpl) GetObjectList(nameExpr string, rtp reflect.Type) ([]any, error) {
	objRef, err := c.beginRead(rtp)
	if err != nil {
		return nil, err
	}
	defer c.mu.RUnlock()

	objects, err := c.objectManager.GetObjects(objRef.RType(), nameExpr, true)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, c.notFoundError(objRef, nameExpr)
	}

	result := make([]any, 0, len(objects))
	for _, object := range objects {
		result = append(result, object.Instance())
	}

	return result, nil
}

func (c *ContainerImpl) GetObjectMap(nameExpr string, rtp reflect.Type) (map[string]any, error) {
	objRef, err := c.beginRead(rtp)
	if err != nil {
		return nil, err
	}
	defer c.mu.RUnlock()

	objects, err := c.objectManager.GetObjects(objRef.RType(), nameExpr, true)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, c.notFoundError(objRef, nameExpr)
	}

	nameToObject := make(map[string]any, len(objects))
	for _, object := range objects {
		if _, exist := nameToObject[object.Name()]; exist {
			return nil, newDuplicateNameObjectError(objRef.RType(), object.Name())
		}
		nameToObject[object.Name()] = object.Instance()
	}

	return nameToObject, nil
}

// beginRead checks the container is started and takes the read lock, which the caller must release.
func (c *ContainerImpl) beginRead(rtp reflect.Type) (ObjectRef, error) {
	objRef, err := parseObjectRef(rtp)
	if err != nil {
		return nil, err
	}
	if err = c.checkReadable(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	if err = c.checkReadable(); err != nil {
		c.mu.RUnlock()
		return nil, err
	}
	return objRef, nil
}

// notFoundError tells apart a missing object from an optional object nobody depended on.
func (c *ContainerImpl) notFoundError(objRef ObjectRef, nameExpr string) error {
	if objects, _ := c.objectManager.GetObjects(objRef.RType(), nameExpr, false); len(objects) > 0 {
		return newObjectNotInitializedError(objRef.FullType(), nameExpr)
	}
	if objRef.RType().Kind() == reflect.Interface {
		return newMissingImplementationError(objRef.FullType(), nameExpr)
	}
	return newMissingObjectError(objRef.FullType(), nameExpr)
}

func (c *ContainerImpl) GetValue(keyExpr string, rtp reflect.Type) (any, bool, error) {
	if err := c.checkReadable(); err != nil {
		return nil, false, err
	}

	return c.valueManager.GetValueWithType(keyExpr, rtp)
}

func (c *ContainerImpl) initObject(ctx *InjectContext, object Object) error {
	if err, ok := c.failed[object]; ok {
		return err
	}

	ctx = NewContext(ctx, object)
	if object.Status() == ObjectStatusInitializing {
		return newCircularDependencyError(ctx)
	}

	object.StartInitialization()
	if err := c.buildObject(ctx, object); err != nil {
		object.ResetInitialization()
		c.failed[object] = err
		return err
	}

	c.initOrder = append(c.initOrder, object)
	return nil
}

func (c *ContainerImpl) buildObject(ctx *InjectContext, object Object) error {
	args := make([]any, 0, len(object.Dependencies()))
	for _, dependency := range object.Dependencies() {
		var arg any
		var err error
		switch dep := dependency.(type) {
		case *objectDependency:
			arg, err = c.getDependencyObjectInstance(ctx, dep)
		case *objectListDependency:
			arg, err = c.getDependencyObjectInstanceList(ctx, dep)
		case *objectMapDependency:
			arg, err = c.getDependencyObjectInstanceMap(ctx, dep)
		case *valueDependency:
			arg, err = c.getDependencyValue(dep)
		default:
			err = newUnsupportedDependencyType(dependency)
		}
		if err != nil {
			return wrapInjectionError(ctx, object, dependency.Location(), err)
		}
		args = append(args, arg)
	}

	if err := object.Build(args); err != nil {
		return wrapInjectionError(ctx, object, "constructor", err)
	}

	if err := object.Init(c.startCtx); err != nil {
		return wrapInjectionError(ctx, object, "init", err)
	}

	return nil
}

// wrapInjectionError adds the object, location and injection stack to err,
// unless err already carries a stack from a deeper dependency.
func wrapInjectionError(ctx *InjectContext, object Object, location string, err error) error {
	var stack InjectionStack
	if errors.As(err, &stack) {
		return err
	}
	return newInjectionError(ctx, object, location, err)
}

func appendUniqueError(errs []error, err error) []error {
	for _, e := range errs {
		if e == err {
			return errs
		}
	}
	return append(errs, err)
}

func (c *ContainerImpl) getDependencyObjectInstance(ctx *InjectContext, dep *objectDependency) (any, error) {
	dependencyObject, err := c.objectManager.GetObject(dep.RType(), dep.NameExpr(), false)
	if err != nil {
		return nil, err
	}

	if dependencyObject == nil {
		if !dep.Optional() {
			return nil, getMissingDependencyError(dep)
		}
		return nil, nil
	}

	if dependencyObject.Status() != ObjectStatusInitialized {
		if err = c.initObject(ctx, dependencyObject); err != nil {
			return nil, err
		}
	}

	return dependencyObject.Instance(), nil
}

func (c *ContainerImpl) getDependencyObjectInstanceList(ctx *InjectContext, dep *objectListDependency) (any, error) {
	implObjects, err := c.objectManager.GetObjects(dep.RType(), dep.NameExpr(), false)
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
			if err = c.initObject(ctx, implObject); err != nil {
				return nil, err
			}
		}
		objectList = reflect.Append(objectList, reflect.ValueOf(implObject.Instance()))
	}

	return objectList.Interface(), nil
}

func (c *ContainerImpl) getDependencyObjectInstanceMap(ctx *InjectContext, dep *objectMapDependency) (any, error) {
	implObjects, err := c.objectManager.GetObjects(dep.RType(), dep.NameExpr(), false)
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
		key := reflect.ValueOf(implObject.Name())
		if objectMap.MapIndex(key).IsValid() {
			return nil, newDuplicateNameObjectError(dep.RType(), implObject.Name())
		}
		if implObject.Status() != ObjectStatusInitialized {
			if err = c.initObject(ctx, implObject); err != nil {
				return nil, err
			}
		}
		objectMap.SetMapIndex(key, reflect.ValueOf(implObject.Instance()))
	}

	return objectMap.Interface(), nil
}

func (c *ContainerImpl) getDependencyValue(dep *valueDependency) (any, error) {
	value, ok, err := c.valueManager.GetValueWithType(dep.NameExpr(), dep.RType())
	if err != nil {
		return nil, err
	}
	if ok {
		return value, nil
	}

	if dep.hasDefaultValue {
		return convertType(dep.defaultValue, dep.RType())
	}
	if !dep.Optional() {
		return nil, newMissingValueError(dep.NameExpr())
	}
	return nil, nil
}

func getMissingDependencyError(dep Dependency) error {
	if dep.isInterface() {
		return newMissingImplementationError(dep.FullType(), dep.NameExpr())
	}
	return newMissingObjectError(dep.FullType(), dep.NameExpr())
}

func (c *ContainerImpl) Dump() string {
	state := c.getState()
	if state != stateStarting {
		// during Start the lock is held by the starting goroutine
		c.mu.RLock()
		defer c.mu.RUnlock()
	}

	objects := c.objectManager.AllObjects()

	var b strings.Builder
	fmt.Fprintf(&b, "container: %s, %d object(s)\n", state, len(objects))
	for _, object := range objects {
		fmt.Fprintf(&b, "* %s", generateFullName(object.FullType(), object.Name()))
		if len(object.Aliases()) > 0 {
			fmt.Fprintf(&b, " aliases=%v", object.Aliases())
		}

		flags := []string{object.Status().String()}
		if object.Provided() {
			flags = append(flags, "provided")
		}
		if object.Primary() {
			flags = append(flags, "primary")
		}
		if object.Optional() {
			flags = append(flags, "optional")
		}
		if object.Condition() != "" {
			if state == stateNew {
				flags = append(flags, fmt.Sprintf("condition=%q", object.Condition()))
			} else {
				flags = append(flags, fmt.Sprintf("condition=%q -> %t", object.Condition(), object.Enabled()))
			}
		}
		fmt.Fprintf(&b, " [%s]\n", strings.Join(flags, ", "))

		for _, dependency := range object.Dependencies() {
			fmt.Fprintf(&b, "    %s <- %s\n", dependency.Location(), describeDependency(dependency))
		}
	}
	return b.String()
}

func describeDependency(dependency Dependency) string {
	var desc string
	switch dep := dependency.(type) {
	case *valueDependency:
		desc = fmt.Sprintf("value %q", dep.NameExpr())
		if dep.hasDefaultValue {
			desc += fmt.Sprintf(" default=%q", dep.defaultValue)
		}
	case *objectListDependency:
		desc = fmt.Sprintf("list %s", generateFullName(dep.FullType(), dep.NameExpr()))
	case *objectMapDependency:
		desc = fmt.Sprintf("map %s", generateFullName(dep.FullType(), dep.NameExpr()))
	default:
		desc = generateFullName(dependency.FullType(), dependency.NameExpr())
	}
	if dependency.Optional() {
		desc += " (optional)"
	}
	return desc
}
