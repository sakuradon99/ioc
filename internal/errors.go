package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	ErrNotStarted     = errors.New("container is not started, call Start first")
	ErrStarting       = errors.New("container is starting, the operation is not allowed during Start (e.g. in Init)")
	ErrAlreadyStarted = errors.New("container is already started")
	ErrShutdown       = errors.New("container is shut down")
)

type InjectionStack interface {
	Stack() string
}

type injectionStack struct {
	ctx *InjectContext
}

func (i *injectionStack) Stack() string {
	if i == nil || i.ctx == nil {
		return ""
	}

	var chain []string
	for current := i.ctx; current != nil; current = current.parentContext {
		if current.initializingObject == nil {
			continue
		}

		fullName := generateFullName(
			current.initializingObject.FullType(),
			current.initializingObject.Name(),
		)
		chain = append(chain, fullName)
	}

	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}

	if len(chain) == 0 {
		return ""
	}

	var builder strings.Builder
	seen := make(map[string]bool, len(chain))
	for idx, fullName := range chain {
		if idx == 0 {
			builder.WriteString("** ")
			builder.WriteString(fullName)
			seen[fullName] = true
			continue
		}
		builder.WriteString("\n")
		if seen[fullName] {
			builder.WriteString("~> ")
		} else {
			builder.WriteString("-> ")
		}
		builder.WriteString(fullName)
		seen[fullName] = true
	}

	return builder.String()
}

// injectionError wraps an error that happened while initializing an object,
// with the object, where it happened (field, constructor param, init...) and the injection stack.
type injectionError struct {
	*injectionStack
	object   Object
	location string
	cause    error
}

func newInjectionError(ctx *InjectContext, object Object, location string, cause error) *injectionError {
	return &injectionError{
		injectionStack: &injectionStack{ctx: ctx},
		object:         object,
		location:       location,
		cause:          cause,
	}
}

func (e *injectionError) Error() string {
	return fmt.Sprintf("<%s> %s: %v", generateFullName(e.object.FullType(), e.object.Name()), e.location, e.cause)
}

func (e *injectionError) Unwrap() error {
	return e.cause
}

type unsupportedRegisterType struct {
	rtp reflect.Type
}

func newUnsupportedRegisterType(rtp reflect.Type) *unsupportedRegisterType {
	return &unsupportedRegisterType{rtp: rtp}
}

func (u *unsupportedRegisterType) Error() string {
	return fmt.Sprintf("unsupported register type <%s>, only struct is allowed", u.rtp.Kind())
}

type objectDuplicateRegisterError struct {
	fullType string
	nameExpr string
}

func newObjectDuplicateRegisterError(fullType string, nameExpr string) *objectDuplicateRegisterError {
	return &objectDuplicateRegisterError{fullType: fullType, nameExpr: nameExpr}
}

func (o *objectDuplicateRegisterError) Error() string {
	return fmt.Sprintf("object <%s> is already registered", generateFullName(o.fullType, o.nameExpr))
}

type missingObjectError struct {
	fullType string
	nameExpr string
}

func newMissingObjectError(fullType string, nameExpr string) *missingObjectError {
	return &missingObjectError{fullType: fullType, nameExpr: nameExpr}
}

func (m *missingObjectError) Error() string {
	return fmt.Sprintf("missing object <%s>", generateFullName(m.fullType, m.nameExpr))
}

type missingImplementationError struct {
	fullType string
	nameExpr string
}

func newMissingImplementationError(fullType string, nameExpr string) *missingImplementationError {
	return &missingImplementationError{fullType: fullType, nameExpr: nameExpr}
}

func (m *missingImplementationError) Error() string {
	return fmt.Sprintf("missing implementation of <%s>", generateFullName(m.fullType, m.nameExpr))
}

type objectNotInitializedError struct {
	fullType string
	nameExpr string
}

func newObjectNotInitializedError(fullType string, nameExpr string) *objectNotInitializedError {
	return &objectNotInitializedError{fullType: fullType, nameExpr: nameExpr}
}

func (e *objectNotInitializedError) Error() string {
	return fmt.Sprintf("object <%s> is not initialized: optional objects are only initialized when depended on",
		generateFullName(e.fullType, e.nameExpr))
}

func foundObjectNames(objects []Object) []string {
	names := make([]string, 0, len(objects))
	for _, object := range objects {
		names = append(names, generateFullName(object.FullType(), object.Name()))
	}
	return names
}

type multipleObjectError struct {
	rtp      reflect.Type
	nameExpr string
	objects  []Object
}

func newMultipleObjectError(rtp reflect.Type, nameExpr string, objects []Object) *multipleObjectError {
	return &multipleObjectError{rtp: rtp, nameExpr: nameExpr, objects: objects}
}

func (m *multipleObjectError) Error() string {
	return fmt.Sprintf("multiple objects found for <%s> %v",
		generateFullName(generateFullType(m.rtp), m.nameExpr), foundObjectNames(m.objects))
}

type multipleImplementationError struct {
	rtp      reflect.Type
	nameExpr string
	objects  []Object
}

func newMultipleImplementationError(rtp reflect.Type, nameExpr string, objects []Object) *multipleImplementationError {
	return &multipleImplementationError{rtp: rtp, nameExpr: nameExpr, objects: objects}
}

func (m *multipleImplementationError) Error() string {
	return fmt.Sprintf("multiple implementations found for <%s> %v",
		generateFullName(generateFullType(m.rtp), m.nameExpr), foundObjectNames(m.objects))
}

type missingValueError struct {
	value string
}

func newMissingValueError(value string) *missingValueError {
	return &missingValueError{value: value}
}

func (m *missingValueError) Error() string {
	return fmt.Sprintf("missing value <%s>", m.value)
}

type unsupportedDependencyType struct {
	dependency Dependency
}

func newUnsupportedDependencyType(dependency Dependency) *unsupportedDependencyType {
	return &unsupportedDependencyType{dependency: dependency}
}

func (u *unsupportedDependencyType) Error() string {
	return fmt.Sprintf("unsupported dependency type [%s]", u.dependency.RType())
}

type circularDependencyError struct {
	*injectionStack
}

func newCircularDependencyError(ctx *InjectContext) *circularDependencyError {
	return &circularDependencyError{
		injectionStack: &injectionStack{
			ctx: ctx,
		},
	}
}

func (c *circularDependencyError) Error() string {
	return "circular dependency detected"
}

type conditionResultNotBoolError struct {
	condition string
}

func newConditionResultNotBoolError(condition string) *conditionResultNotBoolError {
	return &conditionResultNotBoolError{condition: condition}
}

func (c *conditionResultNotBoolError) Error() string {
	return fmt.Sprintf("condition <%s> does not return a bool", c.condition)
}

type unsupportedConstructorError struct {
	constructor any
}

func newUnsupportedConstructorError(constructor any) *unsupportedConstructorError {
	return &unsupportedConstructorError{constructor: constructor}
}

func (u *unsupportedConstructorError) Error() string {
	return fmt.Sprintf("unsupported constructor <%s>, want func(...) *T or func(...) (*T, error)",
		reflect.TypeOf(u.constructor))
}

type constructorNotReturnObjectError struct {
	constructor any
	objectType  reflect.Type
}

func newConstructorNotReturnObjectError(constructor any, objectType reflect.Type) *constructorNotReturnObjectError {
	return &constructorNotReturnObjectError{constructor: constructor, objectType: objectType}
}

func (c *constructorNotReturnObjectError) Error() string {
	return fmt.Sprintf("constructor <%s> does not return <*%s>", reflect.TypeOf(c.constructor), c.objectType)
}

type unsupportedConstructorParamTypeError struct {
	constructor any
	paramType   reflect.Type
}

func newUnsupportedConstructorParamTypeError(constructor any, paramType reflect.Type) *unsupportedConstructorParamTypeError {
	return &unsupportedConstructorParamTypeError{constructor: constructor, paramType: paramType}
}

func (u *unsupportedConstructorParamTypeError) Error() string {
	return fmt.Sprintf("unsupported constructor param type [%s] of <%s>", u.paramType, reflect.TypeOf(u.constructor))
}

type constructorReturnNilError struct {
	constructor any
}

func newConstructorReturnNilError(constructor any) *constructorReturnNilError {
	return &constructorReturnNilError{constructor: constructor}
}

func (e *constructorReturnNilError) Error() string {
	return fmt.Sprintf("constructor <%s> returned nil object without error", reflect.TypeOf(e.constructor))
}

type unsupportedInjectFieldTypeError struct {
	field reflect.StructField
}

func newUnsupportedInjectFieldTypeError(field reflect.StructField) *unsupportedInjectFieldTypeError {
	return &unsupportedInjectFieldTypeError{field: field}
}

func (u *unsupportedInjectFieldTypeError) Error() string {
	return fmt.Sprintf("unsupported inject field type [%s] of field %s", u.field.Type, u.field.Name)
}

type unsupportedObjectTypeError struct {
	rtp reflect.Type
}

func newUnsupportedObjectTypeError(rtp reflect.Type) *unsupportedObjectTypeError {
	return &unsupportedObjectTypeError{rtp: rtp}
}

func (u *unsupportedObjectTypeError) Error() string {
	return fmt.Sprintf("unsupported object type [%s]", generateFullType(u.rtp))
}

type unsupportedObjectRefTypeError struct {
	rtp reflect.Type
}

func newUnsupportedObjectRefTypeError(rtp reflect.Type) *unsupportedObjectRefTypeError {
	return &unsupportedObjectRefTypeError{rtp: rtp}
}

func (u *unsupportedObjectRefTypeError) Error() string {
	return fmt.Sprintf("unsupported object ref type [%s]", generateFullType(u.rtp))
}

type duplicateNameObjectError struct {
	rtp      reflect.Type
	nameExpr string
}

func newDuplicateNameObjectError(rtp reflect.Type, nameExpr string) *duplicateNameObjectError {
	return &duplicateNameObjectError{rtp: rtp, nameExpr: nameExpr}
}

func (d *duplicateNameObjectError) Error() string {
	return fmt.Sprintf(`duplicate name "%s" for type [%s]`, d.nameExpr, generateFullType(d.rtp))
}

type invalidProvideError struct {
	instance any
	reason   string
}

func newInvalidProvideError(instance any, reason string) *invalidProvideError {
	return &invalidProvideError{instance: instance, reason: reason}
}

func (e *invalidProvideError) Error() string {
	return fmt.Sprintf("invalid provided instance of type <%T>: %s", e.instance, e.reason)
}

type valueConvertError struct {
	property any
	rtp      reflect.Type
	cause    error
}

func newValueConvertError(property any, rtp reflect.Type, cause error) *valueConvertError {
	return &valueConvertError{property: property, rtp: rtp, cause: cause}
}

func (e *valueConvertError) Error() string {
	msg := fmt.Sprintf("cannot convert value <%v> of type <%T> to <%s>", e.property, e.property, e.rtp)
	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}
	return msg
}

func (e *valueConvertError) Unwrap() error {
	return e.cause
}
