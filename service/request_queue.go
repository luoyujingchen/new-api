package service

import (
	"container/list"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

const requestQueueThroughputWindow = 60 * time.Second

type PositionNotifier func(position int, estimatedWaitSec int)
type QueueCanProceed func() (bool, error)

type QueueEnqueueOptions struct {
	RequestContext        context.Context
	ModelName             string
	TokenID               int
	CompanyID             int64
	Priority              int
	HeaderTimeoutSeconds  *int
	TokenTimeoutSeconds   int
	CanProceed            QueueCanProceed
	PositionNotifier      PositionNotifier
}

type EffectiveQueueConfig struct {
	ModelName    string `json:"model_name"`
	Enabled      bool   `json:"enabled"`
	MaxQueueSize int    `json:"max_queue_size"`
	QueueTimeout int    `json:"queue_timeout"`
	Configured   bool   `json:"configured"`
}

type QueueStatusSnapshot struct {
	QueueEnabled bool                          `json:"queue_enabled"`
	TotalQueued  int                           `json:"total_queued"`
	Models       map[string]ModelQueueSnapshot `json:"models"`
}

type ModelQueueSnapshot struct {
	Queued        int            `json:"queued"`
	AvgWaitSec    float64        `json:"avg_wait_sec"`
	MaxWaitSec    float64        `json:"max_wait_sec"`
	ThroughputRPM int            `json:"throughput_rpm"`
	MaxQueueSize  int            `json:"max_queue_size"`
	Enabled       bool           `json:"enabled"`
	Buckets       map[string]int `json:"buckets"`
}

type QueueFullError struct {
	ModelName string
	MaxSize   int
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("queue is full for model %s (max size: %d)", e.ModelName, e.MaxSize)
}

type QueuedRequest struct {
	ID        string
	TokenID   int
	CompanyID int64

	ModelName string
	Priority  int

	EnqueuedAt time.Time
	Timeout    time.Duration

	Ready  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	notify     PositionNotifier
	canProceed QueueCanProceed

	position       atomic.Int64
	bucketPriority int
	element        *list.Element
	readyOnce      sync.Once
}

func (r *QueuedRequest) Context() context.Context {
	return r.ctx
}

func (r *QueuedRequest) StopWaiting() {
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *QueuedRequest) Position() int {
	return int(r.position.Load())
}

func (r *QueuedRequest) markReady() {
	r.readyOnce.Do(func() {
		r.position.Store(0)
		if r.notify != nil {
			r.notify(0, 0)
		}
		close(r.Ready)
	})
}

type queueNotification struct {
	req              *QueuedRequest
	position         int
	estimatedWaitSec int
}

type ModelQueue struct {
	modelName string

	mu           sync.Mutex
	buckets      map[int]*list.List
	size         int
	maxQueueSize int
	throughput   []time.Time
}

func newModelQueue(modelName string) *ModelQueue {
	buckets := make(map[int]*list.List, 10)
	for priority := 1; priority <= 10; priority++ {
		buckets[priority] = list.New()
	}
	return &ModelQueue{
		modelName: modelName,
		buckets:   buckets,
	}
}

func (q *ModelQueue) Enqueue(req *QueuedRequest, maxQueueSize int) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.maxQueueSize = maxQueueSize
	if maxQueueSize > 0 && q.size >= maxQueueSize {
		return 0, &QueueFullError{ModelName: q.modelName, MaxSize: maxQueueSize}
	}

	bucket := q.buckets[req.Priority]
	req.bucketPriority = req.Priority
	req.element = bucket.PushBack(req)
	q.size++

	notifications := q.updatePositionsLocked(time.Now())
	position := req.Position()
	go dispatchQueueNotifications(notifications)
	return position, nil
}

func (q *ModelQueue) DequeueByWeight() *QueuedRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	bucketPriority, ok := q.selectBucketByWeightLocked(nil)
	if !ok {
		return nil
	}
	bucket := q.buckets[bucketPriority]
	front := bucket.Front()
	if front == nil {
		return nil
	}
	req, _ := front.Value.(*QueuedRequest)
	q.removeLocked(req)
	q.recordDispatchLocked(time.Now())
	notifications := q.updatePositionsLocked(time.Now())
	go dispatchQueueNotifications(notifications)
	return req
}

func (q *ModelQueue) Remove(req *QueuedRequest) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.removeLocked(req) {
		return false
	}
	notifications := q.updatePositionsLocked(time.Now())
	go dispatchQueueNotifications(notifications)
	return true
}

func (q *ModelQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

func (q *ModelQueue) BucketSizes() map[int]int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.bucketSizesLocked()
}

func (q *ModelQueue) UpdatePositions() {
	q.mu.Lock()
	notifications := q.updatePositionsLocked(time.Now())
	q.mu.Unlock()
	go dispatchQueueNotifications(notifications)
}

func (q *ModelQueue) PeekByWeight(excluded map[int]struct{}) (*QueuedRequest, int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	bucketPriority, ok := q.selectBucketByWeightLocked(excluded)
	if !ok {
		return nil, 0
	}
	front := q.buckets[bucketPriority].Front()
	if front == nil {
		return nil, 0
	}
	req, _ := front.Value.(*QueuedRequest)
	return req, bucketPriority
}

func (q *ModelQueue) DequeueIfHead(req *QueuedRequest) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if req == nil || req.element == nil {
		return false
	}
	bucket := q.buckets[req.bucketPriority]
	if bucket == nil || bucket.Front() != req.element {
		return false
	}
	if !q.removeLocked(req) {
		return false
	}
	q.recordDispatchLocked(time.Now())
	notifications := q.updatePositionsLocked(time.Now())
	go dispatchQueueNotifications(notifications)
	return true
}

func (q *ModelQueue) Snapshot(effectiveConfig EffectiveQueueConfig) ModelQueueSnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	queued := q.size
	buckets := make(map[string]int, 10)
	for priority := 10; priority >= 1; priority-- {
		buckets[fmt.Sprintf("%d", priority)] = q.buckets[priority].Len()
	}

	var totalWait float64
	var maxWait float64
	for priority := 10; priority >= 1; priority-- {
		for node := q.buckets[priority].Front(); node != nil; node = node.Next() {
			req, _ := node.Value.(*QueuedRequest)
			if req == nil {
				continue
			}
			wait := now.Sub(req.EnqueuedAt).Seconds()
			totalWait += wait
			if wait > maxWait {
				maxWait = wait
			}
		}
	}

	avgWait := 0.0
	if queued > 0 {
		avgWait = totalWait / float64(queued)
	}

	return ModelQueueSnapshot{
		Queued:        queued,
		AvgWaitSec:    avgWait,
		MaxWaitSec:    maxWait,
		ThroughputRPM: q.throughputRPMLocked(now),
		MaxQueueSize:  effectiveConfig.MaxQueueSize,
		Enabled:       effectiveConfig.Enabled,
		Buckets:       buckets,
	}
}

func (q *ModelQueue) bucketSizesLocked() map[int]int {
	buckets := make(map[int]int, 10)
	for priority := 1; priority <= 10; priority++ {
		buckets[priority] = q.buckets[priority].Len()
	}
	return buckets
}

func (q *ModelQueue) removeLocked(req *QueuedRequest) bool {
	if req == nil || req.element == nil {
		return false
	}
	bucket := q.buckets[req.bucketPriority]
	if bucket == nil {
		return false
	}
	bucket.Remove(req.element)
	req.element = nil
	if q.size > 0 {
		q.size--
	}
	return true
}

func (q *ModelQueue) selectBucketByWeightLocked(excluded map[int]struct{}) (int, bool) {
	totalWeight := 0
	weightedBuckets := make([]struct {
		priority int
		weight   int
	}, 0, 10)

	for priority := 1; priority <= 10; priority++ {
		if excluded != nil {
			if _, found := excluded[priority]; found {
				continue
			}
		}
		bucketLength := q.buckets[priority].Len()
		if bucketLength == 0 {
			continue
		}
		weight := priority * priority * bucketLength
		totalWeight += weight
		weightedBuckets = append(weightedBuckets, struct {
			priority int
			weight   int
		}{priority: priority, weight: weight})
	}

	if totalWeight == 0 {
		return 0, false
	}

	randWeight, err := cryptoRandInt(totalWeight)
	if err != nil {
		return 0, false
	}

	running := 0
	for _, bucket := range weightedBuckets {
		running += bucket.weight
		if randWeight < running {
			return bucket.priority, true
		}
	}

	last := weightedBuckets[len(weightedBuckets)-1]
	return last.priority, true
}

func (q *ModelQueue) updatePositionsLocked(now time.Time) []queueNotification {
	throughputRPM := q.throughputRPMLocked(now)
	position := 0
	notifications := make([]queueNotification, 0, q.size)

	for priority := 10; priority >= 1; priority-- {
		for node := q.buckets[priority].Front(); node != nil; node = node.Next() {
			req, _ := node.Value.(*QueuedRequest)
			if req == nil {
				continue
			}
			position++
			current := req.Position()
			if current == position {
				continue
			}
			req.position.Store(int64(position))
			notifications = append(notifications, queueNotification{
				req:              req,
				position:         position,
				estimatedWaitSec: estimateQueueWaitSeconds(position, throughputRPM),
			})
		}
	}

	return notifications
}

func (q *ModelQueue) recordDispatchLocked(now time.Time) {
	q.throughput = append(q.throughput, now)
	q.cleanupThroughputLocked(now)
}

func (q *ModelQueue) throughputRPMLocked(now time.Time) int {
	q.cleanupThroughputLocked(now)
	return len(q.throughput)
}

func (q *ModelQueue) cleanupThroughputLocked(now time.Time) {
	cutoff := now.Add(-requestQueueThroughputWindow)
	index := 0
	for index < len(q.throughput) && q.throughput[index].Before(cutoff) {
		index++
	}
	if index > 0 {
		q.throughput = append([]time.Time(nil), q.throughput[index:]...)
	}
}

type RequestQueueService struct {
	mu          sync.RWMutex
	modelQueues map[string]*ModelQueue
	retryEvents chan string
}

var requestQueueService = &RequestQueueService{
	modelQueues: make(map[string]*ModelQueue),
	retryEvents: make(chan string, 256),
}

func GetRequestQueueService() *RequestQueueService {
	return requestQueueService
}

func (s *RequestQueueService) Enqueue(options QueueEnqueueOptions) (*QueuedRequest, int, EffectiveQueueConfig, error) {
	modelName := strings.TrimSpace(options.ModelName)
	if modelName == "" {
		return nil, 0, EffectiveQueueConfig{}, errors.New("model name is required")
	}
	effectiveConfig := s.GetEffectiveQueueConfig(modelName)
	if !effectiveConfig.Enabled {
		return nil, 0, effectiveConfig, errors.New("queue is disabled")
	}

	timeout := s.resolveQueueTimeout(options, effectiveConfig)
	requestContext := options.RequestContext
	if requestContext == nil {
		requestContext = context.Background()
	}
	queueCtx, cancel := context.WithTimeout(requestContext, timeout)

	queuedRequest := &QueuedRequest{
		ID:         common.GetTimeString() + common.GetRandomString(8),
		TokenID:    options.TokenID,
		CompanyID:  options.CompanyID,
		ModelName:  modelName,
		Priority:   setting.NormalizeQueuePriority(options.Priority),
		EnqueuedAt: time.Now(),
		Timeout:    timeout,
		Ready:      make(chan struct{}),
		ctx:        queueCtx,
		cancel:     cancel,
		notify:     options.PositionNotifier,
		canProceed: options.CanProceed,
	}

	queue := s.getOrCreateModelQueue(modelName)
	position, err := queue.Enqueue(queuedRequest, effectiveConfig.MaxQueueSize)
	if err != nil {
		cancel()
		return nil, 0, effectiveConfig, err
	}
	s.NotifySchedulingRetry(modelName)
	return queuedRequest, position, effectiveConfig, nil
}

func (s *RequestQueueService) Remove(req *QueuedRequest) bool {
	if req == nil {
		return false
	}
	queue := s.getModelQueue(req.ModelName)
	if queue == nil {
		return false
	}
	removed := queue.Remove(req)
	if removed {
		s.NotifySchedulingRetry(req.ModelName)
	}
	return removed
}

func (s *RequestQueueService) NotifySchedulingRetry(modelName string) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return
	}
	select {
	case s.retryEvents <- modelName:
	default:
	}
}

func (s *RequestQueueService) GetEffectiveQueueConfig(modelName string) EffectiveQueueConfig {
	modelName = strings.TrimSpace(modelName)
	effectiveConfig := EffectiveQueueConfig{
		ModelName:    modelName,
		Enabled:      setting.QueueEnabled,
		MaxQueueSize: setting.QueueGlobalMaxSize,
		QueueTimeout: 0,
	}
	if !setting.QueueEnabled || modelName == "" {
		return effectiveConfig
	}

	queueConfig, err := model.GetQueueConfigByModelName(modelName)
	if err != nil {
		return effectiveConfig
	}
	effectiveConfig.Configured = true
	effectiveConfig.Enabled = queueConfig.Enabled
	effectiveConfig.MaxQueueSize = queueConfig.MaxQueueSize
	effectiveConfig.QueueTimeout = queueConfig.QueueTimeout
	return effectiveConfig
}

func (s *RequestQueueService) IsQueueEnabledForModel(modelName string) bool {
	return s.GetEffectiveQueueConfig(modelName).Enabled
}

func (s *RequestQueueService) GetStatusSnapshot() QueueStatusSnapshot {
	snapshot := QueueStatusSnapshot{
		QueueEnabled: setting.QueueEnabled,
		Models:       make(map[string]ModelQueueSnapshot),
	}

	for _, modelName := range s.modelNames() {
		queue := s.getModelQueue(modelName)
		if queue == nil {
			continue
		}
		status := queue.Snapshot(s.GetEffectiveQueueConfig(modelName))
		if status.Queued == 0 {
			continue
		}
		snapshot.Models[modelName] = status
		snapshot.TotalQueued += status.Queued
	}
	return snapshot
}

func (s *RequestQueueService) GetModelStatusSnapshot(modelName string) (ModelQueueSnapshot, bool) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return ModelQueueSnapshot{}, false
	}
	queue := s.getModelQueue(modelName)
	if queue == nil {
		return ModelQueueSnapshot{
			Queued:        0,
			AvgWaitSec:    0,
			MaxWaitSec:    0,
			ThroughputRPM: 0,
			MaxQueueSize:  s.GetEffectiveQueueConfig(modelName).MaxQueueSize,
			Enabled:       s.GetEffectiveQueueConfig(modelName).Enabled,
			Buckets:       emptyQueueBuckets(),
		}, true
	}
	return queue.Snapshot(s.GetEffectiveQueueConfig(modelName)), true
}

func (s *RequestQueueService) getModelQueue(modelName string) *ModelQueue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelQueues[modelName]
}

func (s *RequestQueueService) getOrCreateModelQueue(modelName string) *ModelQueue {
	s.mu.RLock()
	queue := s.modelQueues[modelName]
	s.mu.RUnlock()
	if queue != nil {
		return queue
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if queue = s.modelQueues[modelName]; queue != nil {
		return queue
	}
	queue = newModelQueue(modelName)
	s.modelQueues[modelName] = queue
	return queue
}

func (s *RequestQueueService) modelNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	modelNames := make([]string, 0, len(s.modelQueues))
	for modelName := range s.modelQueues {
		modelNames = append(modelNames, modelName)
	}
	sort.Strings(modelNames)
	return modelNames
}

func (s *RequestQueueService) resolveQueueTimeout(options QueueEnqueueOptions, effectiveConfig EffectiveQueueConfig) time.Duration {
	timeoutSeconds := 0
	if options.HeaderTimeoutSeconds != nil && *options.HeaderTimeoutSeconds > 0 {
		timeoutSeconds = *options.HeaderTimeoutSeconds
	} else if options.TokenTimeoutSeconds > 0 {
		timeoutSeconds = options.TokenTimeoutSeconds
	} else if effectiveConfig.QueueTimeout > 0 {
		timeoutSeconds = effectiveConfig.QueueTimeout
	} else {
		timeoutSeconds = setting.QueueDefaultTimeout
	}
	if setting.QueueMaxTimeout > 0 && timeoutSeconds > setting.QueueMaxTimeout {
		timeoutSeconds = setting.QueueMaxTimeout
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 1
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func CalculateQueuePriority(tokenPriority int, companyPriority int) int {
	priority := setting.NormalizeQueuePriority(tokenPriority) + setting.NormalizeQueuePriority(companyPriority) - 5
	switch {
	case priority < 1:
		return 1
	case priority > 10:
		return 10
	default:
		return priority
	}
}

func emptyQueueBuckets() map[string]int {
	buckets := make(map[string]int, 10)
	for priority := 10; priority >= 1; priority-- {
		buckets[fmt.Sprintf("%d", priority)] = 0
	}
	return buckets
}

func estimateQueueWaitSeconds(position int, throughputRPM int) int {
	if position <= 0 || throughputRPM <= 0 {
		return 0
	}
	return int(math.Ceil(float64(position*60) / float64(throughputRPM)))
}

func cryptoRandInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("max must be positive")
	}
	value, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func dispatchQueueNotifications(notifications []queueNotification) {
	for _, notification := range notifications {
		if notification.req == nil || notification.req.notify == nil {
			continue
		}
		notification.req.notify(notification.position, notification.estimatedWaitSec)
	}
}