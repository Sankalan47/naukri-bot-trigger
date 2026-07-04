package naukri

import (
	"fmt"
	"strings"
)

// ErrorKind classifies the domain errors a Client method can return, mirroring
// the Python exception hierarchy (NaukriError -> NaukriAuthError /
// NaukriSessionExpiredError / NaukriSelectorError / NaukriTimeoutError).
type ErrorKind int

const (
	KindUnknown ErrorKind = iota
	KindAuth
	KindSessionExpired
	KindSelector
	KindTimeout
)

// Error is the single error type returned by naukri.Client methods. Every
// raise site in the original Python carries its own formatted message, so a
// struct with a Message field is used instead of fixed sentinel errors.
type Error struct {
	Kind    ErrorKind
	Message string
	Wrapped error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Wrapped
}

func newError(kind ErrorKind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Wrapped: cause}
}

func newAuthError(format string, a ...any) error {
	return newError(KindAuth, fmt.Sprintf(format, a...), nil)
}

func newSessionExpiredError(format string, a ...any) error {
	return newError(KindSessionExpired, fmt.Sprintf(format, a...), nil)
}

func newSelectorError(format string, a ...any) error {
	return newError(KindSelector, fmt.Sprintf(format, a...), nil)
}

func newTimeoutError(cause error, format string, a ...any) error {
	return newError(KindTimeout, fmt.Sprintf(format, a...), cause)
}

// isTimeout reports whether err came from a Playwright wait/goto call
// exceeding its timeout. playwright-go surfaces timeouts as a plain error
// (no distinct exported type across versions), so this matches on the
// message text the driver consistently produces, mirroring the single
// `except PWTimeout` idiom used throughout the original naukri.py.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Timeout") && strings.Contains(err.Error(), "exceeded")
}
