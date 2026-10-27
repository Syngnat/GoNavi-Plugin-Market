package shell

import (
	"errors"
	"fmt"
	"regexp"
)

// Stable error codes. The window UI switches on these, so they must not change
// without a matching UI update.
const (
	codeInvalidArgument       = "invalid_argument"
	codeNotConnected          = "not_connected"
	codeUnsupportedConnection = "unsupported_connection"
	codeUnsupportedTransport  = "unsupported_transport"
	codeConnectionFailed      = "connection_failed"
	codeQueryFailed           = "query_failed"
	codeReadOnly              = "read_only"
	codeScriptError           = "script_error"
	codeSyntaxError           = "syntax_error"
	codeTimeout               = "timeout"
	codeInternal              = "internal"
)

// codedError carries a machine-readable code alongside a message that is safe
// to render in the plugin window.
type codedError struct {
	code    string
	message string
	cause   error
}

func (e *codedError) Error() string { return e.message }

func (e *codedError) Unwrap() error { return e.cause }

func newCodedError(code, message string) error {
	return &codedError{code: code, message: message}
}

func codedErrorf(code, format string, args ...any) error {
	return &codedError{code: code, message: fmt.Sprintf(format, args...)}
}

func wrapCodedError(code, message string, cause error) error {
	return &codedError{code: code, message: message, cause: cause}
}

func errorCode(err error) string {
	var coded *codedError
	if errors.As(err, &coded) && coded.code != "" {
		return coded.code
	}
	return codeInternal
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return redactSecrets(err.Error())
}

// errorEnvelope is the single result shape every command returns. Keeping
// application failures inside the result (instead of returning a Go error)
// lets the window show the message: the host turns handler errors into a bare
// HTTP 502 without the plugin's text.
type errorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type outcome struct {
	OK    bool           `json:"ok"`
	Data  any            `json:"data,omitempty"`
	Error *errorEnvelope `json:"error,omitempty"`
}

func success(data any) outcome {
	return outcome{OK: true, Data: data}
}

func failure(err error) outcome {
	return outcome{OK: false, Error: &errorEnvelope{Code: errorCode(err), Message: errorMessage(err)}}
}

// uriPasswordPattern redacts the password portion of a MongoDB URI so a driver
// parse error can never echo saved credentials to the window.
var uriPasswordPattern = regexp.MustCompile(`(mongodb(?:\+srv)?://[^:/?#\s]+:)([^@\s]+)(@)`)

func redactSecrets(text string) string {
	return uriPasswordPattern.ReplaceAllString(text, "${1}***${3}")
}
