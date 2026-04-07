package ioc

import (
	"time"
)

type InjectContext struct {
	parentContext      *InjectContext
	initializingObject Object
}

func NewContext(parentContext *InjectContext, initializingObject Object) *InjectContext {
	return &InjectContext{parentContext: parentContext, initializingObject: initializingObject}
}

func (c *InjectContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (c *InjectContext) Done() <-chan struct{} {
	return nil
}

func (c *InjectContext) Err() error {
	return nil
}

func (c *InjectContext) Value(key any) any {
	return nil
}
