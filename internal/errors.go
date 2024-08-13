package ioc

import (
	"fmt"
	"reflect"
)

type objectDuplicateRegisterError struct {
	fullType string
	nameExpr string
}

func newObjectDuplicateRegisterError(fullType string, nameExpr string) *objectDuplicateRegisterError {
	return &objectDuplicateRegisterError{fullType: fullType, nameExpr: nameExpr}
}

func (o *objectDuplicateRegisterError) Error() string {
	return fmt.Sprintf("object %s@%s already duplcate register", o.fullType, o.nameExpr)
}

type missingObjectError struct {
	fullType string
	nameExpr string
}

func newMissingObjectError(fullType string, nameExpr string) *missingObjectError {
	return &missingObjectError{fullType: fullType, nameExpr: nameExpr}
}

func (m *missingObjectError) Error() string {
	return fmt.Sprintf("missing object %s@%s", m.fullType, m.nameExpr)
}

type missingImplementationError struct {
	fullType string
	nameExpr string
}

func newMissingImplementationError(fullType string, nameExpr string) *missingImplementationError {
	return &missingImplementationError{fullType: fullType, nameExpr: nameExpr}
}

func (m *missingImplementationError) Error() string {
	return fmt.Sprintf("missing implementation %s@%s", m.fullType, m.nameExpr)
}

type multipleObjectError struct {
	rtp      reflect.Type
	nameExpr string
}

func newMultipleObjectError(rtp reflect.Type, nameExpr string) *multipleObjectError {
	return &multipleObjectError{rtp: rtp, nameExpr: nameExpr}
}

func (m *multipleObjectError) Error() string {
	return fmt.Sprintf("multiple objects found for %s@%s", generateFullType(m.rtp), m.nameExpr)
}

type multipleImplementationError struct {
	rtp      reflect.Type
	nameExpr string
}

func newMultipleImplementationError(rtp reflect.Type, nameExpr string) *multipleImplementationError {
	return &multipleImplementationError{rtp: rtp, nameExpr: nameExpr}
}

func (m *multipleImplementationError) Error() string {
	return fmt.Sprintf("multiple implementations found for %s@%s", generateFullType(m.rtp), m.nameExpr)
}

type missingValueError struct {
	value string
}

func newMissingValueError(value string) *missingValueError {
	return &missingValueError{value: value}
}

func (m *missingValueError) Error() string {
	return fmt.Sprintf("missing value %s", m.value)
}

type unsupportedDependencyType struct {
	dependency Dependency
}

func newUnsupportedDependencyType(dependency Dependency) *unsupportedDependencyType {
	return &unsupportedDependencyType{dependency: dependency}
}

func (u *unsupportedDependencyType) Error() string {
	return fmt.Sprintf("unsupported dependency type %s", u.dependency.RType())
}

type circularDependencyError struct {
}

func newCircularDependencyError() *circularDependencyError {
	return &circularDependencyError{}
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
	return fmt.Sprintf("condition <%s> not return bool", c.condition)
}

type unsupportedConstructorError struct {
	constructor any
}

func newUnsupportedConstructorError(constructor any) *unsupportedConstructorError {
	return &unsupportedConstructorError{constructor: constructor}
}

func (u *unsupportedConstructorError) Error() string {
	return fmt.Sprintf("unsupported constructor type %s", reflect.TypeOf(u.constructor).Kind())
}

type constructorNotReturnObjectError struct {
	constructor any
	objectType  reflect.Type
}

func newConstructorNotReturnObjectError(constructor any, objectType reflect.Type) *constructorNotReturnObjectError {
	return &constructorNotReturnObjectError{constructor: constructor, objectType: objectType}
}

func (c *constructorNotReturnObjectError) Error() string {
	return fmt.Sprintf("constructor %s not return object %s", reflect.TypeOf(c.constructor), c.objectType)
}

type unsupportedConstructorParamTypeError struct {
	constructor any
	paramType   reflect.Type
}

func newUnsupportedConstructorParamTypeError(constructor any, paramType reflect.Type) *unsupportedConstructorParamTypeError {
	return &unsupportedConstructorParamTypeError{constructor: constructor, paramType: paramType}
}

func (u *unsupportedConstructorParamTypeError) Error() string {
	return fmt.Sprintf("unsupported constructor param type %s", u.paramType)
}

type unsupportedInjectFieldTypeError struct {
	field reflect.StructField
}

func newUnsupportedInjectFieldTypeError(field reflect.StructField) *unsupportedInjectFieldTypeError {
	return &unsupportedInjectFieldTypeError{field: field}
}

func (u *unsupportedInjectFieldTypeError) Error() string {
	return fmt.Sprintf("unsupported inject field type %s", u.field.Type.Kind())
}

type unsupportedObjectTypeError struct {
	rtp reflect.Type
}

func newUnsupportedObjectTypeError(rtp reflect.Type) *unsupportedObjectTypeError {
	return &unsupportedObjectTypeError{rtp: rtp}
}

func (u *unsupportedObjectTypeError) Error() string {
	return fmt.Sprintf("unsupported object type %s", generateFullType(u.rtp))
}

type objectRefNotPointerError struct {
	rtp reflect.Type
}

func newObjectRefNotPointerError(rtp reflect.Type) *objectRefNotPointerError {
	return &objectRefNotPointerError{rtp: rtp}
}

func (o *objectRefNotPointerError) Error() string {
	return fmt.Sprintf("object ref <%s> not a pointer", o.rtp.Kind())
}

type unsupportedObjectRefTypeError struct {
	rtp reflect.Type
}

func newUnsupportedObjectRefTypeError(rtp reflect.Type) *unsupportedObjectRefTypeError {
	return &unsupportedObjectRefTypeError{rtp: rtp}
}

func (u *unsupportedObjectRefTypeError) Error() string {
	return fmt.Sprintf("unsupported object ref type <%s>", generateFullType(u.rtp))
}
