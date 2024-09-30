// this file may be moved to a more appropriate location in the future
package utils

// StatusError is an error type which contains a status code.
type StatusError struct {
	Code int
	Msg  string
}

// Error implements the error interface.
func (e *StatusError) Error() string {
	return e.Msg
}
