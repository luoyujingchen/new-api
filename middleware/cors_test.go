package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSPreflightAllowsDashboardHeadersForAnyHTTPOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/api/user/self", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/user/self", nil)
	request.Header.Set("Origin", "https://frontend.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "content-type,cache-control,x-requested-with")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "https://frontend.example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPatch)
	allowHeaders := strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers"))
	require.Contains(t, allowHeaders, "content-type")
	require.Contains(t, allowHeaders, "cache-control")
	require.Contains(t, allowHeaders, "x-requested-with")
}
