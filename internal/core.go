package ioc

import (
	"fmt"
	"reflect"
	"sync"
)

type Container interface {
	Register(ref any, opts ...RegisterOption) error
	GetObject(name string, ref any) (any, error)
}

type ContainerImpl struct {
	objectBuilderFactory *ObjectBuilderFactory
	objectPool           *ObjectPool
	sourceManager        SourceManager
	conditionExecutor    ConditionExecutor
	mu                   sync.Mutex
}

func NewContainerImpl() *ContainerImpl {
	sourceManager := NewSourceManagerImpl()
	conditionExecutor := NewConditionExecutorImpl(sourceManager)
	objectPool := NewObjectPool(conditionExecutor)
	return &ContainerImpl{
		objectBuilderFactory: NewObjectBuilderFactory(),
		objectPool:           objectPool,
		sourceManager:        sourceManager,
		conditionExecutor:    conditionExecutor,
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

	err = c.objectPool.Add(object)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) GetObject(alisa string, ref any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	_, _, objectID, err := parseObjectRef(ref, alisa)
	if err != nil {
		return nil, err
	}

	object, err := c.objectPool.Get(objectID)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("object %s not found", objectID)
	}

	return object.instance, nil
}

func (c *ContainerImpl) load() error {
	requiredObjects := c.objectPool.List()

	for _, object := range requiredObjects {
		if object.inited {
			continue
		}
		if object.optional {
			continue
		}
		if object.condition != "" {
			ok, err := c.conditionExecutor.Execute(object.condition)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}
		err := c.init(object)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) init(object *Object) error {
	if object.initializing {
		return fmt.Errorf("circular dependency detected")
	}
	object.initializing = true

	var args []any

	for _, dependency := range object.dependencies {
		var dependencyObject *Object
		if dependency.IsInterface() {
			implObjects, err := c.objectPool.GetObjectsByInterface(dependency.Type())
			if err != nil {
				return err
			}

			if len(implObjects) == 0 {
				return fmt.Errorf("missing implementation for interface %s", dependency.ID())
			}

			for _, implObject := range implObjects {
				if implObject.alisa != dependency.Alisa() {
					continue
				}

				if dependencyObject != nil {
					return fmt.Errorf("ambiguous implementation for interface %s", dependency.ID())
				}
				dependencyObject = implObject
			}
		} else {
			var err error
			dependencyObject, err = c.objectPool.Get(dependency.ID())
			if err != nil {
				return err
			}
		}

		if dependencyObject == nil {
			if dependency.Optional() {
				args = append(args, nil)
				continue
			}
			return fmt.Errorf("missing dependency object <%s>", dependency.ID())
		}
		if !dependencyObject.inited {
			err := c.init(dependencyObject)
			if err != nil {
				return err
			}
		}
		args = append(args, dependencyObject.instance)
	}

	instance, err := object.instanceBuilder.Build(args)
	if err != nil {
		return err
	}

	it := reflect.TypeOf(instance).Elem()
	for i := 0; i < it.NumField(); i++ {
		field := it.Field(i)
		if valueExpr, ok := field.Tag.Lookup("value"); ok {
			tag := ParseValueTag(valueExpr)
			value, exist, err := c.sourceManager.GetValue(tag.Value())
			if err != nil {
				return err
			}
			if !tag.Optional() && !exist {
				return fmt.Errorf("value <%s> not found", valueExpr)
			}
			if exist {
				assignPrivateField(reflect.ValueOf(instance).Elem().Field(i), value)
			}
		}
	}

	object.instance = instance
	object.inited = true
	object.initializing = false
	return nil
}
