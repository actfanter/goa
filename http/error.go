/*
Package goa standardizes on structured error responses: a request that fails because of an
invalid input or an unexpected condition produces a response that contains a structured error.

The error data structures returned to clients contains five fields: an ID, a code, a status, a
detail and metadata. The ID is unique for the occurrence of the error, it helps correlate the
content of the response with the content of the service logs. The code defines the class of error
(e.g.  "invalid_parameter_type") and the status the corresponding HTTP status (e.g. 400). The detail
contains a message specific to the error occurrence. The metadata contains key/value pairs that
provide contextual information (name of parameters, value of invalid parameter etc.).

Instances of Error can be created via Error Class functions.
See http://goa.design/implement/error_handling.html
All instance of errors created via a error class implement the Error interface. This
interface is leveraged by the error handler middleware to produce the error responses.

The code generated by goagen calls the helper functions exposed in this file when it encounters
invalid data (wrong type, validation errors etc.) such as InvalidParamTypeError,
InvalidAttributeTypeError etc. These methods return errors that get merged with any previously
encountered error via the Error Merge method. The helper functions are error classes stored in
global variable. This means your code can override their values to produce arbitrary error
responses.

goa includes an error handler middleware that takes care of mapping back any error returned by
previously called middleware or action handler into HTTP responses. If the error was created via an
error class then the corresponding content including the HTTP status is used otherwise an internal
error is returned. Errors that bubble up all the way to the top (i.e. not handled by the error
middleware) also generate an internal error response.
*/
package goa

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/goadesign/goa"
)

var (
	// ErrorMediaIdentifier is the media type identifier used for error responses.
	ErrorMediaIdentifier = "application/vnd.goa.error"

	// ErrBadRequest is a generic bad request error.
	ErrBadRequest = NewErrorClass("bad_request", 400)

	// ErrUnauthorized is a generic unauthorized error.
	ErrUnauthorized = NewErrorClass("unauthorized", 401)

	// ErrInvalidRequest is the class of errors produced by the generated code when a request
	// parameter or payload fails to validate.
	ErrInvalidRequest = NewErrorClass("invalid_request", 400)

	// ErrInvalidEncoding is the error produced when a request body fails to be decoded.
	ErrInvalidEncoding = NewErrorClass("invalid_encoding", 400)

	// ErrRequestBodyTooLarge is the error produced when the size of a request body exceeds
	// MaxRequestBodyLength bytes.
	ErrRequestBodyTooLarge = NewErrorClass("request_too_large", 413)

	// ErrInvalidFile is the error produced by ServeFiles when requested to serve non-existant
	// or non-readable files.
	ErrInvalidFile = NewErrorClass("invalid_file", 404)

	// ErrNotFound is the error returned to requests that don't match a registered handler.
	ErrNotFound = NewErrorClass("not_found", 404)

	// ErrInternal is the class of error used for uncaught errors.
	ErrInternal = NewErrorClass("internal", 500)
)

type (
	// ErrorResponse contains the details of a error response.
	// This struct is mainly intended for clients to decode error responses.
	ErrorResponse struct {
		// ID is the unique error instance identifier.
		ID string `json:"id" xml:"id" form:"id"`
		// Code identifies the class of errors.
		Code string `json:"code" xml:"code" form:"code"`
		// Status is the HTTP status code used by responses that cary the error.
		Status int `json:"status" xml:"status" form:"status"`
		// Detail describes the specific error occurrence.
		Detail string `json:"detail" xml:"detail" form:"detail"`
		// Meta contains additional key/value pairs useful to clients.
		Meta []map[string]interface{} `json:"meta,omitempty" xml:"meta,omitempty" form:"meta,omitempty"`
	}
)

// NewErrorResponse creates a HTTP response from the given goa Error.
func NewErrorResponse(err goa.Error) *ErrorResponse {
}

// NewErrorClass creates a new error class.
// It is the responsibility of the client to guarantee uniqueness of code.
func NewErrorClass(code string, status int) ErrorClass {
	return func(message interface{}, keyvals ...interface{}) error {
		var msg string
		switch actual := message.(type) {
		case string:
			msg = actual
		case error:
			msg = actual.Error()
		case fmt.Stringer:
			msg = actual.String()
		default:
			msg = fmt.Sprintf("%v", actual)
		}
		meta := make([]map[string]interface{}, (len(keyvals)+1)/2)
		for i := 0; i < len(keyvals); i += 2 {
			k := keyvals[i]
			var v interface{} = "MISSING"
			if i+1 < len(keyvals) {
				v = keyvals[i+1]
			}
			meta[i/2] = map[string]interface{}{fmt.Sprintf("%v", k): v}
		}
		return &ErrorResponse{ID: newErrorID(), Code: code, Status: status, Detail: msg, Meta: meta}
	}
}

// MissingPayloadError is the error produced when a request is missing a required payload.
func MissingPayloadError() error {
	return ErrInvalidRequest("missing required payload")
}

// InvalidParamTypeError is the error produced when the type of a parameter does not match the type
// defined in the design.
func InvalidParamTypeError(name string, val interface{}, expected string) error {
	msg := fmt.Sprintf("invalid value %#v for parameter %#v, must be a %s", val, name, expected)
	return ErrInvalidRequest(msg, "param", name, "value", val, "expected", expected)
}

// MissingParamError is the error produced for requests that are missing path or querystring
// parameters.
func MissingParamError(name string) error {
	msg := fmt.Sprintf("missing required parameter %#v", name)
	return ErrInvalidRequest(msg, "name", name)
}

// InvalidAttributeTypeError is the error produced when the type of payload field does not match
// the type defined in the design.
func InvalidAttributeTypeError(ctx string, val interface{}, expected string) error {
	msg := fmt.Sprintf("type of %s must be %s but got value %#v", ctx, expected, val)
	return ErrInvalidRequest(msg, "attribute", ctx, "value", val, "expected", expected)
}

// MissingAttributeError is the error produced when a request payload is missing a required field.
func MissingAttributeError(ctx, name string) error {
	msg := fmt.Sprintf("attribute %#v of %s is missing and required", name, ctx)
	return ErrInvalidRequest(msg, "attribute", name, "parent", ctx)
}

// MissingHeaderError is the error produced when a request is missing a required header.
func MissingHeaderError(name string) error {
	msg := fmt.Sprintf("missing required HTTP header %#v", name)
	return ErrInvalidRequest(msg, "name", name)
}

// InvalidEnumValueError is the error produced when the value of a parameter or payload field does
// not match one the values defined in the design Enum validation.
func InvalidEnumValueError(ctx string, val interface{}, allowed []interface{}) error {
	elems := make([]string, len(allowed))
	for i, a := range allowed {
		elems[i] = fmt.Sprintf("%#v", a)
	}
	msg := fmt.Sprintf("value of %s must be one of %s but got value %#v", ctx, strings.Join(elems, ", "), val)
	return ErrInvalidRequest(msg, "attribute", ctx, "value", val, "expected", strings.Join(elems, ", "))
}

// InvalidFormatError is the error produced when the value of a parameter or payload field does not
// match the format validation defined in the design.
func InvalidFormatError(ctx, target string, format Format, formatError error) error {
	msg := fmt.Sprintf("%s must be formatted as a %s but got value %#v, %s", ctx, format, target, formatError.Error())
	return ErrInvalidRequest(msg, "attribute", ctx, "value", target, "expected", format, "error", formatError.Error())
}

// InvalidPatternError is the error produced when the value of a parameter or payload field does
// not match the pattern validation defined in the design.
func InvalidPatternError(ctx, target string, pattern string) error {
	msg := fmt.Sprintf("%s must match the regexp %#v but got value %#v", ctx, pattern, target)
	return ErrInvalidRequest(msg, "attribute", ctx, "value", target, "regexp", pattern)
}

// InvalidRangeError is the error produced when the value of a parameter or payload field does
// not match the range validation defined in the design.
func InvalidRangeError(ctx string, target interface{}, value int, min bool) error {
	comp := "greater or equal"
	if !min {
		comp = "lesser or equal"
	}
	msg := fmt.Sprintf("%s must be %s than %d but got value %#v", ctx, comp, value, target)
	return ErrInvalidRequest(msg, "attribute", ctx, "value", target, "comp", comp, "expected", value)
}

// InvalidLengthError is the error produced when the value of a parameter or payload field does
// not match the length validation defined in the design.
func InvalidLengthError(ctx string, target interface{}, ln, value int, min bool) error {
	comp := "greater or equal"
	if !min {
		comp = "lesser or equal"
	}
	msg := fmt.Sprintf("length of %s must be %s than %d but got value %#v (len=%d)", ctx, comp, value, target, ln)
	return ErrInvalidRequest(msg, "attribute", ctx, "value", target, "len", ln, "comp", comp, "expected", value)
}

// NoAuthMiddleware is the error produced when goa is unable to lookup a auth middleware for a
// security scheme defined in the design.
func NoAuthMiddleware(schemeName string) error {
	msg := fmt.Sprintf("Auth middleware for security scheme %s is not mounted", schemeName)
	return ErrNoAuthMiddleware(msg, "scheme", schemeName)
}

// Error returns the error occurrence details.
func (e *ErrorResponse) Error() string {
	msg := fmt.Sprintf("[%s] %d %s: %s", e.ID, e.Status, e.Code, e.Detail)
	for _, val := range e.Meta {
		for k, v := range val {
			msg += ", " + fmt.Sprintf("%s: %v", k, v)
		}
	}
	return msg
}

// ResponseStatus is the status used to build responses.
func (e *ErrorResponse) ResponseStatus() int { return e.Status }

// Token is the unique error occurrence identifier.
func (e *ErrorResponse) Token() string { return e.ID }

// MergeErrors updates an error by merging another into it. It first converts other into a
// Error if not already one - producing an internal error in that case. The merge algorithm
// is:
//
// * If any of e or other is an internal error then the result is an internal error
//
// * If the status or code of e and other don't match then the result is a 400 "bad_request"
//
// The Detail field is updated by concatenating the Detail fields of e and other separated
// by a semi-colon. The MetaValues field of is updated by merging the map of other MetaValues
// into e's where values in e with identical keys to values in other get overwritten.
//
// Merge returns the updated error. This is useful in case the error was initially nil in
// which case other is returned.
func MergeErrors(err, other error) error {
	if err == nil {
		if other == nil {
			return nil
		}
		return asErrorResponse(other)
	}
	if other == nil {
		return asErrorResponse(err)
	}
	e := asErrorResponse(err)
	o := asErrorResponse(other)
	switch {
	case e.Status == 500 || o.Status == 500:
		if e.Status != 500 {
			e.Status = 500
			e.Code = "internal_error"
		}
	case e.Status != o.Status || e.Code != o.Code:
		e.Status = 400
		e.Code = "bad_request"
	}
	e.Detail = e.Detail + "; " + o.Detail

	for _, val := range o.Meta {
		for k, v := range val {
			e.Meta = append(e.Meta, map[string]interface{}{k: v})
		}
	}
	return e
}

func asErrorResponse(err error) *ErrorResponse {
	e, ok := err.(*ErrorResponse)
	if !ok {
		return &ErrorResponse{Status: 500, Code: "internal_error", Detail: err.Error()}
	}
	return e
}

// If you're curious - simplifying a bit - the probability of 2 values being equal for n 6-bytes
// values is n^2 / 2^49. For n = 1 million this gives around 1 chance in 500. 6 bytes seems to be a
// good trade-off between probability of clashes and length of ID (6 * 4/3 = 8 chars) since clashes
// are not catastrophic.
func newErrorID() string {
	b := make([]byte, 6)
	io.ReadFull(rand.Reader, b)
	return base64.StdEncoding.EncodeToString(b)
}
