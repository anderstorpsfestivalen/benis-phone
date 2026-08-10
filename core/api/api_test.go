package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/gin-gonic/gin"
)

func TestHTTPAuthenticationReadsLiveCredentialStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", currentBasicAuth(), func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	secrets.Replace(secrets.Credentials{HTTPServerAuth: secrets.PWCombo{
		Username: "first",
		Password: "password-one",
	}})
	if status := authStatus(router, "first", "password-one"); status != http.StatusNoContent {
		t.Fatalf("initial auth status = %d", status)
	}
	secrets.Replace(secrets.Credentials{HTTPServerAuth: secrets.PWCombo{
		Username: "second",
		Password: "password-two",
	}})
	if status := authStatus(router, "first", "password-one"); status != http.StatusUnauthorized {
		t.Fatalf("old auth status after reload = %d", status)
	}
	if status := authStatus(router, "second", "password-two"); status != http.StatusNoContent {
		t.Fatalf("new auth status after reload = %d", status)
	}
}

func authStatus(handler http.Handler, username, password string) int {
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Code
}
