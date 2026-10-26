package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/validator/keymanager"
)

// Errors a remote signer response maps to, one per status code of the
// contract. Callers match them with errors.Is on the returned *SignerError.
var (
	ErrBadRequest           = errors.New("remote signer rejected the request as invalid")
	ErrUnauthenticated      = errors.New("remote signer rejected credentials")
	ErrForbidden            = errors.New("remote signer refused the request by policy")
	ErrPublicKeyNotFound    = errors.New("public key not found")
	ErrRouteNotFound        = errors.New("remote signer has no such route: check the remote signer url")
	ErrSlashingProtection   = errors.New("remote signer refused the request: slashing protection")
	ErrRequestTooLarge      = errors.New("remote signer rejected the request body as too large")
	ErrUnsupportedMediaType = errors.New("remote signer rejected the request content type")
	ErrRandaoRevealRefused  = errors.New("remote signer cannot reveal this randao commitment")
	ErrRateLimited          = errors.New("remote signer rate limited the request")
	ErrSignerInternal       = errors.New("remote signer internal error")
	ErrSignerUnavailable    = errors.New("remote signer unavailable")
	ErrUnexpectedResponse   = errors.New("unexpected remote signer response")
)

const (
	reasonUnknownKey              = "UNKNOWN_KEY"
	reasonRandaoUnsupportedForKey = "RANDAO_UNSUPPORTED_FOR_KEY"
)

// SignerError is a non-2xx response of the remote signer. It carries the
// signer's machine-readable reason and unwraps to one of the Err* values above
// (or to keymanager.ErrRandaoRevealUnsupported).
type SignerError struct {
	StatusCode int
	ReasonCode string
	Message    string
	// RetryAfter is the parsed Retry-After header, zero when absent.
	RetryAfter time.Duration
	err        error
}

var _ keymanager.Reasoner = (*SignerError)(nil)

// errorBody is the signer's error envelope.
type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func newSignerError(status int, header http.Header, body []byte) *SignerError {
	e := &SignerError{StatusCode: status}
	var b errorBody
	if err := json.Unmarshal(body, &b); err == nil {
		e.ReasonCode = b.Reason
		e.Message = b.Message
	}
	e.err = statusError(status, e.ReasonCode)
	if secs, err := strconv.ParseUint(header.Get("Retry-After"), 10, 32); err == nil {
		e.RetryAfter = time.Duration(secs) * time.Second
	}
	return e
}

// statusError maps a status code, and for 404 and 422 the reason, to an error.
// A 404 is a missing key only with the UNKNOWN_KEY reason: without a reason it
// is the signer's (or a proxy's) answer to a path that matches no route, which
// a wrong url prefix causes.
func statusError(status int, reason string) error {
	switch status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusUnauthorized:
		return ErrUnauthenticated
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		switch reason {
		case reasonUnknownKey:
			return ErrPublicKeyNotFound
		case "":
			return ErrRouteNotFound
		default:
			return ErrUnexpectedResponse
		}
	case http.StatusPreconditionFailed:
		return ErrSlashingProtection
	case http.StatusRequestEntityTooLarge:
		return ErrRequestTooLarge
	case http.StatusUnsupportedMediaType:
		return ErrUnsupportedMediaType
	case http.StatusUnprocessableEntity:
		if reason == reasonRandaoUnsupportedForKey {
			return keymanager.ErrRandaoRevealUnsupported
		}
		return ErrRandaoRevealRefused
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusInternalServerError:
		return ErrSignerInternal
	case http.StatusServiceUnavailable:
		return ErrSignerUnavailable
	default:
		return ErrUnexpectedResponse
	}
}

// Error implements error. The message names the status and reason only, never
// the request URL or body.
func (e *SignerError) Error() string {
	msg := fmt.Sprintf("%v (status %d", e.err, e.StatusCode)
	if e.ReasonCode != "" {
		msg += ", reason " + e.ReasonCode
	}
	return msg + ")"
}

// Reason returns the signer's machine-readable refusal reason, empty when the
// response did not carry one.
func (e *SignerError) Reason() string {
	return e.ReasonCode
}

// Is reports a 412 protection refusal as keymanager.ErrSigningRefused, so
// callers outside this package can tell it from a fault.
func (e *SignerError) Is(target error) bool {
	return target == keymanager.ErrSigningRefused && e.StatusCode == http.StatusPreconditionFailed
}

// Unwrap returns the error the status code maps to.
func (e *SignerError) Unwrap() error {
	return e.err
}

// logFields returns the fields logged for a refused request. They never include
// the request body, headers or URL.
func (e *SignerError) logFields(route string) logrus.Fields {
	return logrus.Fields{
		"route":  route,
		"status": e.StatusCode,
		"reason": e.ReasonCode,
	}
}

func (e *SignerError) log(route string) {
	entry := log.WithFields(e.logFields(route))
	switch e.StatusCode {
	case http.StatusPreconditionFailed, http.StatusTooManyRequests:
		entry.Warn("Remote signer refused the request")
	default:
		entry.Error("Remote signer request failed")
	}
}
