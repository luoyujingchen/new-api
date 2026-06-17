package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type payloadCaptureWriter struct {
	gin.ResponseWriter
	capture *model.UsageLogPayloadCapture
}

func (w *payloadCaptureWriter) Write(data []byte) (int, error) {
	if w.capture != nil && model.ShouldCaptureUsageLogPayloadBody(w.Header()) {
		w.capture.CaptureClientResponseWrite(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *payloadCaptureWriter) WriteString(s string) (int, error) {
	if w.capture != nil && model.ShouldCaptureUsageLogPayloadBody(w.Header()) {
		w.capture.CaptureClientResponseWrite([]byte(s))
	}
	return w.ResponseWriter.WriteString(s)
}

func (w *payloadCaptureWriter) WriteHeader(statusCode int) {
	if w.capture != nil {
		w.capture.CaptureClientResponseHeaders(w.Header())
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func UsageLogPayloadCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.GetEnvOrDefaultBool("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", false) {
			c.Next()
			return
		}
		capture := model.StartUsageLogPayloadCapture(c)
		if capture != nil {
			c.Writer = &payloadCaptureWriter{
				ResponseWriter: c.Writer,
				capture:        capture,
			}
		}
		c.Next()
		if capture != nil {
			capture.CaptureClientResponseHeaders(c.Writer.Header())
			if c.Writer.Status() >= http.StatusBadRequest && c.Errors != nil && len(c.Errors) > 0 {
				capture.CaptureUpstreamError(c.Errors.Last())
			}
			model.FinalizeUsageLogPayloadCapture(c)
		}
	}
}

func UsageLogPayloadRequestCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.GetEnvOrDefaultBool("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", false) {
			c.Next()
			return
		}
		if capture := model.StartUsageLogPayloadCapture(c); capture != nil {
			capture.CaptureClientRequestFromGin(c)
		}
		c.Next()
	}
}
