package model

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

const (
	defaultPayloadCaptureMaxBytes = 2 << 20
)

type UsageLogPayloadCapture struct {
	mu                sync.Mutex
	maxBytes          int
	payload           UsageLogPayload
	clientResponseBuf *limitedCaptureBuffer
}

type limitedCaptureBuffer struct {
	limit     int
	size      int
	truncated bool
	buf       bytes.Buffer
}

func NewUsageLogPayloadCapture(maxBytes int) *UsageLogPayloadCapture {
	if maxBytes <= 0 {
		maxBytes = defaultPayloadCaptureMaxBytes
	}
	return &UsageLogPayloadCapture{
		maxBytes:          maxBytes,
		clientResponseBuf: newLimitedCaptureBuffer(maxBytes),
	}
}

func StartUsageLogPayloadCapture(c *gin.Context) *UsageLogPayloadCapture {
	if c == nil {
		return nil
	}
	if capture, ok := common.GetContextKeyType[*UsageLogPayloadCapture](c, constant.ContextKeyUsageLogPayloadCapture); ok && capture != nil {
		return capture
	}
	maxBytes := common.GetEnvOrDefault("USAGE_LOG_PAYLOAD_CAPTURE_MAX_BYTES", defaultPayloadCaptureMaxBytes)
	capture := NewUsageLogPayloadCapture(maxBytes)
	common.SetContextKey(c, constant.ContextKeyUsageLogPayloadCapture, capture)
	return capture
}

func GetUsageLogPayloadCapture(c *gin.Context) (*UsageLogPayloadCapture, bool) {
	if c == nil {
		return nil, false
	}
	return common.GetContextKeyType[*UsageLogPayloadCapture](c, constant.ContextKeyUsageLogPayloadCapture)
}

func FinalizeUsageLogPayloadCapture(c *gin.Context) {
	capture, ok := GetUsageLogPayloadCapture(c)
	if !ok || capture == nil {
		return
	}
	capture.FinalizeClientResponse()
	payload := capture.Payload()
	if payload.PayloadSizeBytes > 0 || payload.CaptureMode != "" {
		SetUsageLogPayload(c, payload)
		if eventID := common.GetContextKeyString(c, constant.ContextKeyUsageLogOutboxEventID); eventID != "" {
			if err := RefreshUsageLogOutboxPayload(eventID, payload); err != nil {
				common.SysLog("failed to refresh usage log outbox payload: " + err.Error())
			}
		}
	}
}

func (c *UsageLogPayloadCapture) CaptureClientRequest(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	headers := headerJSON(req.Header)
	body := ""
	truncated := false
	if ShouldCaptureUsageLogPayloadBody(req.Header) {
		body, truncated = c.captureRequestBodyPrefix(req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload.ClientRequestHeadersJson = headers
	c.payload.ClientRequestBody = body
	if truncated {
		c.payload.Truncated = true
	}
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) CaptureClientRequestFromGin(ctx *gin.Context) {
	if c == nil || ctx == nil || ctx.Request == nil {
		return
	}
	headers := headerJSON(ctx.Request.Header)
	body := ""
	truncated := false
	if ShouldCaptureUsageLogPayloadBody(ctx.Request.Header) {
		if storage, ok := existingBodyStorage(ctx); ok {
			body, truncated = c.captureBodyStoragePrefix(storage)
			_, _ = storage.Seek(0, io.SeekStart)
			ctx.Request.Body = io.NopCloser(storage)
		} else {
			body, truncated = c.captureRequestBodyPrefix(ctx.Request)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload.ClientRequestHeadersJson = headers
	c.payload.ClientRequestBody = body
	if truncated {
		c.payload.Truncated = true
	}
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) CaptureUpstreamRequest(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	headers := headerJSON(req.Header)
	body := ""
	truncated := false
	if ShouldCaptureUsageLogPayloadBody(req.Header) {
		body, truncated = c.captureRequestBodyPrefix(req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload.UpstreamRequestHeadersJson = headers
	c.payload.UpstreamRequestBody = body
	if truncated {
		c.payload.Truncated = true
	}
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) CaptureUpstreamResponse(resp *http.Response) {
	if c == nil || resp == nil {
		return
	}
	headers := headerJSON(resp.Header)
	c.mu.Lock()
	c.payload.UpstreamResponseHeadersJson = headers
	c.recomputeSizeLocked()
	c.mu.Unlock()
	if resp.Body != nil && ShouldCaptureUsageLogPayloadBody(resp.Header) {
		resp.Body = &captureReadCloser{
			ReadCloser: resp.Body,
			capture:    c,
		}
	}
}

func (c *UsageLogPayloadCapture) CaptureUpstreamError(err error) {
	if c == nil || err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload.ErrorBody = c.truncateStringLocked(err.Error())
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) CaptureClientResponseWrite(data []byte) {
	if c == nil || len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.clientResponseBuf.Write(data)
}

func (c *UsageLogPayloadCapture) CaptureClientResponseHeaders(header http.Header) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload.ClientResponseHeadersJson = headerJSON(header)
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) FinalizeClientResponse() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clientResponseBuf != nil {
		c.payload.ClientResponseBody = c.clientResponseBuf.String()
		if c.clientResponseBuf.Truncated() {
			c.payload.Truncated = true
		}
	}
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) Payload() UsageLogPayload {
	if c == nil {
		return UsageLogPayload{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	payload := c.payload
	if payload.CaptureMode == "" {
		payload.CaptureMode = c.captureMode()
	}
	return sanitizeUsageLogPayload(payload)
}

func (c *UsageLogPayloadCapture) appendUpstreamResponse(data []byte) {
	if c == nil || len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	existing := c.payload.UpstreamResponseBody
	buf := newLimitedCaptureBuffer(c.maxBytes)
	_, _ = buf.Write([]byte(existing))
	_, _ = buf.Write(data)
	c.payload.UpstreamResponseBody = buf.String()
	if buf.Truncated() {
		c.payload.Truncated = true
	}
	c.recomputeSizeLocked()
}

func (c *UsageLogPayloadCapture) truncateStringLocked(value string) string {
	if c.maxBytes <= 0 || len(value) <= c.maxBytes {
		return value
	}
	c.payload.Truncated = true
	return value[:c.maxBytes]
}

func (c *UsageLogPayloadCapture) captureRequestBodyPrefix(req *http.Request) (string, bool) {
	if c == nil || req == nil {
		return "", false
	}
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err != nil || rc == nil {
			return "", false
		}
		defer rc.Close()
		body, _, truncated, err := readPayloadPrefix(rc, c.maxBytes)
		if err != nil {
			return "", false
		}
		return string(body), truncated
	}
	if req.Body == nil {
		return "", false
	}
	body, replay, truncated, err := readPayloadPrefix(req.Body, c.maxBytes)
	req.Body = &replayReadCloser{
		reader: io.MultiReader(bytes.NewReader(replay), req.Body),
		closer: req.Body,
	}
	if err != nil {
		return "", false
	}
	return string(body), truncated
}

func (c *UsageLogPayloadCapture) captureBodyStoragePrefix(storage common.BodyStorage) (string, bool) {
	if c == nil || storage == nil {
		return "", false
	}
	limit := c.maxBytes
	if limit <= 0 {
		limit = defaultPayloadCaptureMaxBytes
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return "", false
	}
	readLimit := int64(limit)
	if storage.Size() >= 0 && storage.Size() < readLimit {
		readLimit = storage.Size()
	}
	data, err := io.ReadAll(io.LimitReader(storage, readLimit))
	if err != nil {
		return "", false
	}
	return string(data), storage.Size() > int64(limit)
}

func existingBodyStorage(ctx *gin.Context) (common.BodyStorage, bool) {
	if ctx == nil {
		return nil, false
	}
	storage, exists := ctx.Get(common.KeyBodyStorage)
	if !exists || storage == nil {
		return nil, false
	}
	bodyStorage, ok := storage.(common.BodyStorage)
	return bodyStorage, ok
}

func readPayloadPrefix(reader io.Reader, maxBytes int) (capture []byte, replay []byte, truncated bool, err error) {
	if reader == nil {
		return nil, nil, false, nil
	}
	if maxBytes <= 0 {
		maxBytes = defaultPayloadCaptureMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
	if len(data) > maxBytes {
		return data[:maxBytes], data, true, err
	}
	return data, data, false, err
}

func (c *UsageLogPayloadCapture) recomputeSizeLocked() {
	size := len(c.payload.ClientRequestHeadersJson) +
		len(c.payload.ClientRequestBody) +
		len(c.payload.UpstreamRequestHeadersJson) +
		len(c.payload.UpstreamRequestBody) +
		len(c.payload.UpstreamResponseHeadersJson) +
		len(c.payload.UpstreamResponseBody) +
		len(c.payload.ClientResponseHeadersJson) +
		len(c.payload.ClientResponseBody) +
		len(c.payload.ErrorBody)
	c.payload.PayloadSizeBytes = size
	if c.payload.CaptureMode == "" {
		c.payload.CaptureMode = c.captureMode()
	}
}

func (c *UsageLogPayloadCapture) captureMode() string {
	if c == nil {
		return "text:" + strconv.Itoa(defaultPayloadCaptureMaxBytes)
	}
	maxBytes := c.maxBytes
	if maxBytes <= 0 {
		maxBytes = defaultPayloadCaptureMaxBytes
	}
	return "text:" + strconv.Itoa(maxBytes)
}

type captureReadCloser struct {
	io.ReadCloser
	capture *UsageLogPayloadCapture
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.capture.appendUpstreamResponse(p[:n])
	}
	return n, err
}

type replayReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *replayReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *replayReadCloser) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func newLimitedCaptureBuffer(limit int) *limitedCaptureBuffer {
	if limit <= 0 {
		limit = defaultPayloadCaptureMaxBytes
	}
	return &limitedCaptureBuffer{limit: limit}
}

func (b *limitedCaptureBuffer) Write(p []byte) (int, error) {
	b.size += len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedCaptureBuffer) String() string {
	if b == nil {
		return ""
	}
	return b.buf.String()
}

func (b *limitedCaptureBuffer) Truncated() bool {
	return b != nil && b.truncated
}

func headerJSON(header http.Header) string {
	if len(header) == 0 {
		return ""
	}
	data := make(map[string]interface{}, len(header))
	for key, values := range header {
		if len(values) == 1 {
			data[key] = values[0]
		} else {
			copied := make([]string, len(values))
			copy(copied, values)
			data[key] = copied
		}
	}
	bytes, err := common.Marshal(data)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func CaptureModeFromEnv() string {
	if !common.GetEnvOrDefaultBool("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", false) {
		return "disabled"
	}
	return "text:" + strconv.Itoa(common.GetEnvOrDefault("USAGE_LOG_PAYLOAD_CAPTURE_MAX_BYTES", defaultPayloadCaptureMaxBytes))
}

func ShouldCaptureUsageLogPayloadBody(header http.Header) bool {
	if len(header) == 0 {
		return false
	}
	return isTextPayloadContentType(header.Get("Content-Type"))
}

func isTextPayloadContentType(contentType string) bool {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/json",
		"application/x-ndjson",
		"application/xml",
		"application/x-www-form-urlencoded",
		"application/javascript",
		"application/graphql",
		"application/yaml",
		"application/x-yaml",
		"application/csv":
		return true
	default:
		return false
	}
}
