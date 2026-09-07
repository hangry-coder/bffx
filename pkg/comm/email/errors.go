package email

import "errors"

// ErrResendNonOK is returned when the Resend HTTP API responds with a non-2xx status.
var ErrResendNonOK = errors.New("email: resend request failed")
