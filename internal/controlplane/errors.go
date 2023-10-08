package controlplane

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// notFoundError is an error indicating the resource is not found.
type notFoundError struct {
	err error
}

// Error calls the underlying error's Error method.
func (n *notFoundError) Error() string {
	return fmt.Sprintf("not found: %s", n.err.Error())
}

// NotFound indicates that this is a not found error.
func (n *notFoundError) NotFound() bool {
	return true
}

// NewNotFound wraps an existing error as a not found error.
func NewNotFound(err error) error {
	return &notFoundError{
		err: err,
	}
}

// notFound indicates a resource is not found.
type notFound interface {
	NotFound() bool
}

// IsNotFound checks whether an error implements the notFound interface.
func IsNotFound(err error) bool {
	var nferr notFound
	return errors.As(err, &nferr) && nferr.NotFound()
}
