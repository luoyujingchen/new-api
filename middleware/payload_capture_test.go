package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type trackingReadCloser struct {
	reader *strings.Reader
	reads  int
}

func newTrackingReadCloser(raw string) *trackingReadCloser {
	return &trackingReadCloser{reader: strings.NewReader(raw)}
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	r.reads++
	return r.reader.Read(p)
}

func (r *trackingReadCloser) Close() error {
	return nil
}

func TestUsageLogPayloadCaptureDoesNotReadBodyBeforeRequestCapture(t *testing.T) {
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(UsageLogPayloadCapture())
	router.POST("/relay",
		func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
		},
		UsageLogPayloadRequestCapture(),
		func(c *gin.Context) {
			t.Fatal("request capture handler should not run after abort")
		},
	)

	body := newTrackingReadCloser(`{"message":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/relay", body)
	request.Body = body
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, 0, body.reads)
}

func TestUsageLogPayloadCaptureDisabledByDefault(t *testing.T) {
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", "")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(UsageLogPayloadCapture())
	router.POST("/relay",
		UsageLogPayloadRequestCapture(),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)

	body := newTrackingReadCloser(`{"message":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/relay", body)
	request.Body = body
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, 0, body.reads)
}

func TestUsageLogPayloadRequestCaptureReadsBodyAfterAllowedMiddleware(t *testing.T) {
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(UsageLogPayloadCapture())
	router.POST("/relay",
		func(c *gin.Context) {
			c.Next()
		},
		UsageLogPayloadRequestCapture(),
		func(c *gin.Context) {
			_, _ = io.ReadAll(c.Request.Body)
			c.Status(http.StatusNoContent)
		},
	)

	body := newTrackingReadCloser(`{"message":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/relay", body)
	request.Body = body
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Greater(t, body.reads, 0)
}
