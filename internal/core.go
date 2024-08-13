package ioc

import (
	"reflect"
	"sync"
)

type Container interface {
	Register(ref any, opts ...RegisterOption) error
	GetObject(nameExpr string, ref any) (any, error)
	GetObjects(nameExpr string, ref any) ([]any, error)
}

type ContainerImpl struct {
	objectBuilderFactory ObjectBuilderFactory
	objectPool           ObjectPool
	sourceManager        PropertyManager
	mu                   sync.Mutex
}

func NewContainerImpl() *ContainerImpl {
	sourceManager := newPropertyManagerImpl()
	conditionExecutor := newConditionExecutorImpl(sourceManager)
	objectPool := newObjectPoolImpl(conditionExecutor)
	return &ContainerImpl{
		objectBuilderFactory: newObjectBuilderFactoryImpl(),
		objectPool:           objectPool,
		sourceManager:        sourceManager,
	}
}

func (c *ContainerImpl) Register(ref any, opts ...RegisterOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	ob := c.objectBuilderFactory.GetBuilder(options)
	object, err := ob.Build(ref, options)
	if err != nil {
		return err
	}

	err = c.objectPool.AddObject(object)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) GetObject(nameExpr string, ref any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	objRef, err := parseObjectRef(ref)
	if err != nil {
		return nil, err
	}

	object, err := c.objectPool.GetObject(objRef.RType(), nameExpr)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, newMissingObjectError(objRef.FullType(), nameExpr)
	}

	return object.Instance(), nil
}

func (c *ContainerImpl) GetObjects(nameExpr string, ref any) ([]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	objRef, err := parseObjectRef(ref)
	if err != nil {
		return nil, err
	}

	objects, err := c.objectPool.GetObjects(objRef.RType(), nameExpr)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, newMissingObjectError(objRef.FullType(), nameExpr)
	}

	var result []any
	for _, object := range objects {
		result = append(result, object.Instance())
	}

	return result, nil
}

func (c *ContainerImpl) load() error {
	requiredObjects, err := c.objectPool.ListObjects()
	if err != nil {
		return err
	}

	for _, object := range requiredObjects {
		if object.Status() == ObjectStatusInitialized {
			continue
		}
		if object.Optional() {
			continue
		}
		err = c.initObject(object)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) initObject(object Object) error {
	if object.Status() == ObjectStatusInitializing {
		return newCircularDependencyError()
	}
	object.StartInitialization()

	var args []any

	for _, dependency := range object.Dependencies() {
		switch dependency.(type) {
		case *objectDependency:
			dep := dependency.(*objectDependency)
			dependencyObject, err := c.objectPool.GetObject(dep.RType(), dep.NameExpr())
			if err != nil {
				return err
			}

			if dependencyObject == nil {
				if !dependency.Optional() {
					if dep.isInterface() {
						return newMissingImplementationError(dep.FullType(), dep.NameExpr())
					}
					return newMissingObjectError(dep.FullType(), dep.NameExpr())
				}
				args = append(args, nil)
				continue
			}
			if dependencyObject.Status() != ObjectStatusInitialized {
				err = c.initObject(dependencyObject)
				if err != nil {
					return err
				}
			}
			args = append(args, dependencyObject.Instance())
		case *objectsDependency:
			dep := dependency.(*objectsDependency)
			implObjects, err := c.objectPool.GetObjects(dep.RType(), dep.NameExpr())
			if err != nil {
				return err
			}

			objectList := reflect.MakeSlice(dep.SliceType(), 0, len(implObjects))
			for _, implObject := range implObjects {
				if implObject.Status() != ObjectStatusInitialized {
					err = c.initObject(implObject)
					if err != nil {
						return err
					}
				}
				objectList = reflect.Append(objectList, reflect.ValueOf(implObject.Instance()))
			}
			args = append(args, objectList.Interface())
		case *valueDependency:
			v, ok, err := c.sourceManager.GetPropertyWithType(dependency.NameExpr(), dependency.RType())
			if err != nil {
				return err
			}
			if !ok && !dependency.Optional() {
				return newMissingValueError(dependency.NameExpr())
			}
			args = append(args, v)
		default:
			return newUnsupportedDependencyType(dependency)
		}
	}

	_, err := object.Build(args)
	if err != nil {
		return err
	}

	return nil
}
