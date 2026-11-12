package ioc

import "context"

// Initializer is called after the object is created and injected.
type Initializer interface {
	Init() error
}

// ContextInitializer is like Initializer, receiving the context passed to Start.
type ContextInitializer interface {
	Init(ctx context.Context) error
}

// Destroyer is called on Shutdown, in reverse initialization order.
type Destroyer interface {
	Destroy() error
}

// ContextDestroyer is like Destroyer, receiving the context passed to Shutdown.
type ContextDestroyer interface {
	Destroy(ctx context.Context) error
}

func initInstance(ctx context.Context, instance any) error {
	switch obj := instance.(type) {
	case ContextInitializer:
		return obj.Init(ctx)
	case Initializer:
		return obj.Init()
	}
	return nil
}

func destroyInstance(ctx context.Context, instance any) error {
	switch obj := instance.(type) {
	case ContextDestroyer:
		return obj.Destroy(ctx)
	case Destroyer:
		return obj.Destroy()
	}
	return nil
}
