// this file may be moved to a more appropriate location in the future
package utils

import (
	"io"
	"net/http"
)

// StatusError is an error type which contains a status code.
type StatusError struct {
	Code int
	Msg  string
}

// Error implements the error interface.
func (e *StatusError) Error() string {
	return e.Msg
}

// FromHTTPResponse constructs a StatusError from an HTTP response.
func FromHTTPResponse(resp *http.Response) *StatusError {
	body, _ := io.ReadAll(resp.Body)
	return &StatusError{
		Code: resp.StatusCode,
		Msg:  string(body),
	}
}
