package apimiddleware

import (
	"net/http"
	"testing"

	"github.com/theQRL/qrysm/testing/assert"
)

var _ error = (*DefaultErrorJson)(nil)

func TestDefaultErrorJson_Error(t *testing.T) {
	err := &DefaultErrorJson{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
	}
	assert.Equal(t, "error 500: internal server error", err.Error())
}
