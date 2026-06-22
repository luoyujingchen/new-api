package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetApiRouterHandlesUserSelfPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	request := httptest.NewRequest(http.MethodOptions, "/api/user/self", nil)
	request.Header.Set("Origin", "https://frontend.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "content-type,cache-control,x-requested-with")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "https://frontend.example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
	require.Contains(t, strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers")), "x-requested-with")
}
