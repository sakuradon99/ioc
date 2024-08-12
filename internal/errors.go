package ioc

import "fmt"

type missingObjectError struct {
	objectID string
}

func newMissingObjectError(objectID string) *missingObjectError {
	return &missingObjectError{objectID: objectID}
}

func (m *missingObjectError) Error() string {
	return fmt.Sprintf("missing dependency object: %s", m.objectID)
}

type missingImplementationError struct {
	interfaceFullType string
}

func newMissingImplementationError(interfaceFullType string) *missingImplementationError {
	return &missingImplementationError{interfaceFullType: interfaceFullType}
}

func (m *missingImplementationError) Error() string {
	return fmt.Sprintf("missing dependency implementation for interface: %s", m.interfaceFullType)
}
