package xerrors

import (
	"io"
	"net/http"
)

// StatusError is an error with a status code.
type StatusError struct {
	StatusCode int
	Msg        string
}

// Error implements the error interface.
func (e *StatusError) Error() string {
	return e.Msg
}

// New constructs a StatusError.
func New(code int, msg string) error {
	return &StatusError{StatusCode: code, Msg: msg}
}

// FromHTTPResponse constructs a StatusError from an HTTP response.
func FromHTTPResponse(resp *http.Response) error {
	// ignore the read error?
	msg, _ := io.ReadAll(resp.Body)
	return &StatusError{StatusCode: resp.StatusCode, Msg: string(msg)}
}

// Wrap wraps an error with a status code.
func Wrap(err error, code int) error {
	if err == nil {
		return nil
	}

	if se, ok := err.(*StatusError); ok {
		return se
	}

	return &StatusError{StatusCode: code, Msg: err.Error()}
}
