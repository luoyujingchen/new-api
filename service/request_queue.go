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
	"github.com/QuantumNous/new-api/types"
)

const requestQueueThroughputWindow = 60 * time.Second

type PositionNotifier func(position int, estimatedWaitSec int)
type QueueCanProceed func() (bool, error)

type QueueEnqueueOptions struct {
	RequestContext        context.Context
	ModelName             string
	TokenID               int
	CompanyID             int64
	DepartmentID          int64
	Priority              int
	HeaderTimeoutSeconds  *int
	TokenTimeoutSeconds   int
	EstimatedPromptTokens int
	LongContextTiers      []types.QueueLongContextTier
	RetainedLeaseID       string
	CanProceed            QueueCanProceed
	PositionNotifier      PositionNotifier
}

type EffectiveQueueConfig struct {
	ModelName        string                       `json:"model_name"`
	Enabled          bool                         `json:"enabled"`
	MaxQueueSize     int                          `json:"max_queue_size"`
	QueueTimeout     int                          `json:"queue_timeout"`
	Configured       bool                         `json:"configured"`
	LongContextTiers []types.QueueLongContextTier `json:"long_context_tiers"`
}

type QueueStatusSnapshot struct {
	QueueEnabled bool                          `json:"queue_enabled"`
	TotalQueued  int                           `json:"total_queued"`
	Models       map[string]ModelQueueSnapshot `json:"models"`
}

type ModelQueueSnapshot struct {
	Queued           int                                `json:"queued"`
	AvgWaitSec       float64                            `json:"avg_wait_sec"`
	MaxWaitSec       float64                            `json:"max_wait_sec"`
	ThroughputRPM    int                                `json:"throughput_rpm"`
	MaxQueueSize     int                                `json:"max_queue_size"`
	Enabled          bool                               `json:"enabled"`
	Buckets          map[string]int                     `json:"buckets"`
	LongContextTiers []types.QueueLongContextTierStatus `json:"long_context_tiers"`
}

type QueueFullError struct {
	ModelName string
	MaxSize   int
}

type LongContextLeaseUseOptions struct {
	ModelName        string
	TokenID          int
	CompanyID        int64
	LongContextTiers []types.QueueLongContextTier
}

type LongContextLeaseUse struct {
	ID                 string
	OwnerKey           string
	RemainingTurns     int
	IdleTimeoutSeconds int
	LeaseTurns         int
	ThresholdTokens    int
	CreatedAt          time.Time
	LastUsedAt         time.Time
	IdleExpiresAt      time.Time
	leaseIdleTimeout   time.Duration
	holder             *QueuedRequest
	timer              *time.Timer
	active             bool
}

type LongContextTaskSnapshot struct {
	ID                    string `json:"id"`
	Kind                  string `json:"kind"`
	ModelName             string `json:"model_name"`
	TokenID               int    `json:"token_id"`
	CompanyID             int64  `json:"company_id"`
	DepartmentID          int64  `json:"department_id,omitempty"`
	CompanyName           string `json:"company_name,omitempty"`
	DepartmentName        string `json:"department_name,omitempty"`
	ThresholdTokens       int    `json:"threshold_tokens"`
	EstimatedPromptTokens int    `json:"estimated_prompt_tokens"`
	Priority              int    `json:"priority"`
	Status                string `json:"status"`
	CreatedAt             int64  `json:"created_at"`
	WaitSeconds           int    `json:"wait_seconds"`
	RemainingTurns        int    `json:"remaining_turns"`
	LeaseTurns            int    `json:"lease_turns"`
	IdleTimeoutSeconds    int    `json:"idle_timeout_seconds"`
	IdleExpiresAt         int64  `json:"idle_expires_at,omitempty"`
	Active                bool   `json:"active"`
}

type LongContextTasksSnapshot struct {
	Items []LongContextTaskSnapshot `json:"items"`
	Total int                       `json:"total"`
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("queue is full for model %s (max size: %d)", e.ModelName, e.MaxSize)
}

func (l *LongContextLeaseUse) snapshot() *LongContextLeaseUse {
	if l == nil {
		return nil
	}
	return &LongContextLeaseUse{
		ID:                 l.ID,
		OwnerKey:           l.OwnerKey,
		RemainingTurns:     l.RemainingTurns,
		IdleTimeoutSeconds: l.IdleTimeoutSeconds,
		LeaseTurns:         l.LeaseTurns,
		ThresholdTokens:    l.ThresholdTokens,
		CreatedAt:          l.CreatedAt,
		LastUsedAt:         l.LastUsedAt,
		IdleExpiresAt:      l.IdleExpiresAt,
	}
}

func (l *LongContextLeaseUse) matches(options LongContextLeaseUseOptions) bool {
	if l == nil || l.holder == nil {
		return false
	}
	return l.holder.ModelName == strings.TrimSpace(options.ModelName) &&
		l.holder.TokenID == options.TokenID &&
		l.holder.CompanyID == options.CompanyID
}

type QueuedRequest struct {
	ID           string
	TokenID      int
	CompanyID    int64
	DepartmentID int64

	ModelName             string
	Priority              int
	EstimatedPromptTokens int
	LongContextTiers      []types.QueueLongContextTier

	EnqueuedAt time.Time
	Timeout    time.Duration

	Ready  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	notify     PositionNotifier
	canProceed QueueCanProceed

	position                 atomic.Int64
	adminCancelled           atomic.Bool
	bucketPriority           int
	element                  *list.Element
	readyOnce                sync.Once
	longContextSlotsAcquired bool
	retainedLeaseID          string
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

func (r *QueuedRequest) IsAdminCancelled() bool {
	if r == nil {
		return false
	}
	return r.adminCancelled.Load()
}

func (r *QueuedRequest) RetainedLeaseID() string {
	if r == nil {
		return ""
	}
	return r.retainedLeaseID
}

func (r *QueuedRequest) cancelByAdmin() {
	if r == nil {
		return
	}
	r.adminCancelled.Store(true)
	if r.cancel != nil {
		r.cancel()
	}
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

	mu                 sync.Mutex
	buckets            map[int]*list.List
	size               int
	maxQueueSize       int
	throughput         []time.Time
	longContextRunning map[int]int
}

func newModelQueue(modelName string) *ModelQueue {
	buckets := make(map[int]*list.List, 10)
	for priority := 1; priority <= 10; priority++ {
		buckets[priority] = list.New()
	}
	return &ModelQueue{
		modelName:          modelName,
		buckets:            buckets,
		longContextRunning: make(map[int]int),
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

func (q *ModelQueue) AcquireCandidateByWeight(excludedBuckets map[int]struct{}, excludedRequests map[*QueuedRequest]struct{}, currentTiers []types.QueueLongContextTier) (*QueuedRequest, int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	bucketPriority, ok := q.selectBucketByWeightLocked(excludedBuckets)
	if !ok {
		return nil, 0, false
	}
	bucket := q.buckets[bucketPriority]
	if bucket == nil || bucket.Len() == 0 {
		return nil, bucketPriority, true
	}
	for node := bucket.Front(); node != nil; node = node.Next() {
		req, _ := node.Value.(*QueuedRequest)
		if req == nil {
			continue
		}
		if excludedRequests != nil {
			if _, found := excludedRequests[req]; found {
				continue
			}
		}
		if len(req.LongContextTiers) == 0 {
			req.LongContextTiers = MatchLongContextTiers(req.EstimatedPromptTokens, currentTiers)
		}
		if !q.canAcquireLongContextSlotsLocked(req) {
			continue
		}
		q.acquireLongContextSlotsLocked(req)
		return req, bucketPriority, true
	}
	return nil, bucketPriority, true
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

func (q *ModelQueue) Dequeue(req *QueuedRequest) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

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
		Queued:           queued,
		AvgWaitSec:       avgWait,
		MaxWaitSec:       maxWait,
		ThroughputRPM:    q.throughputRPMLocked(now),
		MaxQueueSize:     effectiveConfig.MaxQueueSize,
		Enabled:          effectiveConfig.Enabled,
		Buckets:          buckets,
		LongContextTiers: q.longContextTierStatusLocked(effectiveConfig.LongContextTiers),
	}
}

func (q *ModelQueue) HasLongContextRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, running := range q.longContextRunning {
		if running > 0 {
			return true
		}
	}
	return false
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

func (q *ModelQueue) canAcquireLongContextSlotsLocked(req *QueuedRequest) bool {
	if req == nil || len(req.LongContextTiers) == 0 || req.longContextSlotsAcquired || req.retainedLeaseID != "" {
		return true
	}
	for _, tier := range req.LongContextTiers {
		if tier.MaxRunning <= 0 {
			continue
		}
		if q.longContextRunning[tier.ThresholdTokens] >= tier.MaxRunning {
			return false
		}
	}
	return true
}

func (q *ModelQueue) acquireLongContextSlotsLocked(req *QueuedRequest) {
	if req == nil || len(req.LongContextTiers) == 0 || req.longContextSlotsAcquired || req.retainedLeaseID != "" {
		return
	}
	for _, tier := range req.LongContextTiers {
		q.longContextRunning[tier.ThresholdTokens]++
	}
	req.longContextSlotsAcquired = true
}

func (q *ModelQueue) ReleaseLongContextSlots(req *QueuedRequest) bool {
	if req == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if !req.longContextSlotsAcquired {
		return false
	}
	for _, tier := range req.LongContextTiers {
		current := q.longContextRunning[tier.ThresholdTokens]
		if current <= 1 {
			delete(q.longContextRunning, tier.ThresholdTokens)
			continue
		}
		q.longContextRunning[tier.ThresholdTokens] = current - 1
	}
	req.longContextSlotsAcquired = false
	return true
}

func (q *ModelQueue) CancelQueuedRequest(id string) (*QueuedRequest, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, false
	}

	q.mu.Lock()
	var req *QueuedRequest
	for priority := 1; priority <= 10 && req == nil; priority++ {
		for node := q.buckets[priority].Front(); node != nil; node = node.Next() {
			candidate, _ := node.Value.(*QueuedRequest)
			if candidate != nil && candidate.ID == id {
				req = candidate
				break
			}
		}
	}
	if req == nil || !q.removeLocked(req) {
		q.mu.Unlock()
		return nil, false
	}
	notifications := q.updatePositionsLocked(time.Now())
	q.mu.Unlock()

	req.cancelByAdmin()
	go dispatchQueueNotifications(notifications)
	return req, true
}

func (q *ModelQueue) LongContextQueuedSnapshots(tiers []types.QueueLongContextTier) []LongContextTaskSnapshot {
	if len(tiers) == 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	items := make([]LongContextTaskSnapshot, 0)
	for priority := 10; priority >= 1; priority-- {
		for node := q.buckets[priority].Front(); node != nil; node = node.Next() {
			req, _ := node.Value.(*QueuedRequest)
			if req == nil {
				continue
			}
			matched := MatchLongContextTiers(req.EstimatedPromptTokens, tiers)
			tier, ok := highestLongContextTier(matched)
			if !ok {
				continue
			}
			items = append(items, LongContextTaskSnapshot{
				ID:                    req.ID,
				Kind:                  "queued",
				ModelName:             req.ModelName,
				TokenID:               req.TokenID,
				CompanyID:             req.CompanyID,
				DepartmentID:          req.DepartmentID,
				ThresholdTokens:       tier.ThresholdTokens,
				EstimatedPromptTokens: req.EstimatedPromptTokens,
				Priority:              req.Priority,
				Status:                "waiting",
				CreatedAt:             req.EnqueuedAt.Unix(),
				WaitSeconds:           secondsSince(req.EnqueuedAt, now),
			})
		}
	}
	return items
}

func (q *ModelQueue) longContextTierStatusLocked(tiers []types.QueueLongContextTier) []types.QueueLongContextTierStatus {
	if len(tiers) == 0 {
		return nil
	}
	queuedByThreshold := make(map[int]int, len(tiers))
	for priority := 1; priority <= 10; priority++ {
		for node := q.buckets[priority].Front(); node != nil; node = node.Next() {
			req, _ := node.Value.(*QueuedRequest)
			if req == nil {
				continue
			}
			for _, tier := range MatchLongContextTiers(req.EstimatedPromptTokens, tiers) {
				queuedByThreshold[tier.ThresholdTokens]++
			}
		}
	}
	status := make([]types.QueueLongContextTierStatus, 0, len(tiers))
	for _, tier := range tiers {
		status = append(status, types.QueueLongContextTierStatus{
			ThresholdTokens:         tier.ThresholdTokens,
			MaxRunning:              tier.MaxRunning,
			LeaseTurns:              tier.LeaseTurns,
			LeaseIdleTimeoutSeconds: tier.LeaseIdleTimeoutSeconds,
			Running:                 q.longContextRunning[tier.ThresholdTokens],
			Queued:                  queuedByThreshold[tier.ThresholdTokens],
		})
	}
	return status
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
	mu                sync.RWMutex
	modelQueues       map[string]*ModelQueue
	retryEvents       chan string
	leaseMu           sync.Mutex
	longContextLeases map[string]*LongContextLeaseUse
	longContextOwners map[string]map[string]struct{}
}

var requestQueueService = &RequestQueueService{
	modelQueues:       make(map[string]*ModelQueue),
	retryEvents:       make(chan string, 256),
	longContextLeases: make(map[string]*LongContextLeaseUse),
	longContextOwners: make(map[string]map[string]struct{}),
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
		ID:                    common.GetTimeString() + common.GetRandomString(8),
		TokenID:               options.TokenID,
		CompanyID:             options.CompanyID,
		DepartmentID:          options.DepartmentID,
		ModelName:             modelName,
		Priority:              setting.NormalizeQueuePriority(options.Priority),
		EstimatedPromptTokens: options.EstimatedPromptTokens,
		LongContextTiers:      options.LongContextTiers,
		retainedLeaseID:       strings.TrimSpace(options.RetainedLeaseID),
		EnqueuedAt:            time.Now(),
		Timeout:               timeout,
		Ready:                 make(chan struct{}),
		ctx:                   queueCtx,
		cancel:                cancel,
		notify:                options.PositionNotifier,
		canProceed:            options.CanProceed,
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

func (s *RequestQueueService) ReleaseLongContextSlots(req *QueuedRequest) bool {
	if req == nil {
		return false
	}
	queue := s.getModelQueue(req.ModelName)
	if queue == nil {
		return false
	}
	released := queue.ReleaseLongContextSlots(req)
	if released {
		s.NotifySchedulingRetry(req.ModelName)
	}
	return released
}

func longContextLeaseOwnerKey(modelName string, tokenID int, companyID int64, tiers []types.QueueLongContextTier) (string, types.QueueLongContextTier, bool) {
	tier, ok := highestLongContextTier(tiers)
	if !ok {
		return "", types.QueueLongContextTier{}, false
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || tokenID == 0 {
		return "", types.QueueLongContextTier{}, false
	}
	return fmt.Sprintf("%d:%d:%s:%d", tokenID, companyID, modelName, tier.ThresholdTokens), tier, true
}

func (s *RequestQueueService) StartLongContextLease(req *QueuedRequest) *LongContextLeaseUse {
	if req == nil || !req.longContextSlotsAcquired || len(req.LongContextTiers) == 0 {
		return nil
	}
	ownerKey, tier, ok := longContextLeaseOwnerKey(req.ModelName, req.TokenID, req.CompanyID, req.LongContextTiers)
	if !ok {
		return nil
	}
	leaseTurns := tier.LeaseTurns
	if leaseTurns <= 0 {
		leaseTurns = types.DefaultQueueLongContextLeaseTurns
	}
	idleTimeoutSeconds := tier.LeaseIdleTimeoutSeconds
	if idleTimeoutSeconds <= 0 {
		idleTimeoutSeconds = types.DefaultQueueLongContextLeaseIdleTimeoutSeconds
	}
	remainingTurns := leaseTurns - 1
	if remainingTurns <= 0 {
		return nil
	}

	now := time.Now()
	lease := &LongContextLeaseUse{
		ID:                 common.GetTimeString() + common.GetRandomString(16),
		OwnerKey:           ownerKey,
		RemainingTurns:     remainingTurns,
		IdleTimeoutSeconds: idleTimeoutSeconds,
		LeaseTurns:         leaseTurns,
		ThresholdTokens:    tier.ThresholdTokens,
		CreatedAt:          now,
		LastUsedAt:         now,
		leaseIdleTimeout:   time.Duration(idleTimeoutSeconds) * time.Second,
		holder:             req,
		active:             true,
	}

	s.leaseMu.Lock()
	s.ensureLongContextLeaseMapLocked()
	s.longContextLeases[lease.ID] = lease
	s.addLongContextLeaseOwnerLocked(lease)
	s.leaseMu.Unlock()
	return lease.snapshot()
}

func (s *RequestQueueService) TryBeginLongContextLeaseUse(options LongContextLeaseUseOptions) (*LongContextLeaseUse, bool) {
	ownerKey, _, ok := longContextLeaseOwnerKey(options.ModelName, options.TokenID, options.CompanyID, options.LongContextTiers)
	if !ok {
		return nil, false
	}

	s.leaseMu.Lock()
	defer s.leaseMu.Unlock()

	ownerLeaseIDs := s.longContextOwners[ownerKey]
	if len(ownerLeaseIDs) == 0 {
		return nil, false
	}
	ids := make([]string, 0, len(ownerLeaseIDs))
	for id := range ownerLeaseIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	now := time.Now()
	for _, id := range ids {
		lease := s.longContextLeases[id]
		if lease == nil || lease.active || lease.RemainingTurns <= 0 {
			continue
		}
		if !lease.matches(options) || !coversLongContextTiers(lease.holder.LongContextTiers, options.LongContextTiers) {
			continue
		}
		if lease.timer != nil {
			lease.timer.Stop()
			lease.timer = nil
		}
		lease.active = true
		lease.RemainingTurns--
		lease.LastUsedAt = now
		lease.IdleExpiresAt = time.Time{}
		return lease.snapshot(), true
	}
	return nil, false
}

func (s *RequestQueueService) CancelLongContextLeaseUse(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}

	s.leaseMu.Lock()
	defer s.leaseMu.Unlock()

	lease := s.longContextLeases[id]
	if lease == nil || !lease.active {
		return
	}
	if lease.RemainingTurns < lease.LeaseTurns-1 {
		lease.RemainingTurns++
	}
	lease.active = false
	s.scheduleLongContextLeaseIdleReleaseLocked(lease)
}

func (s *RequestQueueService) FinishLongContextLeaseUse(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}

	s.leaseMu.Lock()
	lease := s.longContextLeases[id]
	if lease == nil {
		s.leaseMu.Unlock()
		return false
	}
	lease.active = false
	lease.LastUsedAt = time.Now()
	if lease.RemainingTurns > 0 {
		s.scheduleLongContextLeaseIdleReleaseLocked(lease)
		s.leaseMu.Unlock()
		return false
	}
	holder := lease.holder
	if lease.timer != nil {
		lease.timer.Stop()
	}
	delete(s.longContextLeases, id)
	s.removeLongContextLeaseOwnerLocked(lease)
	s.leaseMu.Unlock()
	return s.ReleaseLongContextSlots(holder)
}

func (s *RequestQueueService) scheduleLongContextLeaseIdleReleaseLocked(lease *LongContextLeaseUse) {
	if lease == nil {
		return
	}
	if lease.timer != nil {
		lease.timer.Stop()
	}
	id := lease.ID
	timeout := lease.leaseIdleTimeout
	if timeout <= 0 {
		timeout = time.Duration(types.DefaultQueueLongContextLeaseIdleTimeoutSeconds) * time.Second
	}
	lease.IdleExpiresAt = time.Now().Add(timeout)
	lease.timer = time.AfterFunc(timeout, func() {
		s.releaseLongContextLease(id)
	})
}

func (s *RequestQueueService) releaseLongContextLease(id string) bool {
	s.leaseMu.Lock()
	lease := s.longContextLeases[id]
	if lease == nil || lease.active {
		s.leaseMu.Unlock()
		return false
	}
	holder := lease.holder
	delete(s.longContextLeases, id)
	s.removeLongContextLeaseOwnerLocked(lease)
	s.leaseMu.Unlock()
	return s.ReleaseLongContextSlots(holder)
}

func (s *RequestQueueService) CancelLongContextQueuedRequest(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, modelName := range s.modelNames() {
		queue := s.getModelQueue(modelName)
		if queue == nil {
			continue
		}
		if req, ok := queue.CancelQueuedRequest(id); ok {
			if req != nil && req.retainedLeaseID != "" {
				s.CancelLongContextLeaseUse(req.retainedLeaseID)
			}
			s.NotifySchedulingRetry(modelName)
			return true
		}
	}
	return false
}

func (s *RequestQueueService) CancelLongContextTask(kind string, id string) bool {
	switch strings.TrimSpace(kind) {
	case "queued":
		return s.CancelLongContextQueuedRequest(id)
	default:
		return false
	}
}

func (s *RequestQueueService) GetLongContextTasksSnapshot(modelName string) LongContextTasksSnapshot {
	modelName = strings.TrimSpace(modelName)
	items := make([]LongContextTaskSnapshot, 0)

	for _, currentModelName := range s.modelNames() {
		if modelName != "" && currentModelName != modelName {
			continue
		}
		queue := s.getModelQueue(currentModelName)
		if queue == nil {
			continue
		}
		effectiveConfig := s.GetEffectiveQueueConfig(currentModelName)
		items = append(items, queue.LongContextQueuedSnapshots(effectiveConfig.LongContextTiers)...)
	}

	now := time.Now()
	s.leaseMu.Lock()
	for _, lease := range s.longContextLeases {
		if lease == nil || lease.holder == nil {
			continue
		}
		if modelName != "" && lease.holder.ModelName != modelName {
			continue
		}
		status := "idle"
		if lease.active {
			status = "active"
		}
		idleExpiresAt := int64(0)
		if !lease.IdleExpiresAt.IsZero() {
			idleExpiresAt = lease.IdleExpiresAt.Unix()
		}
		items = append(items, LongContextTaskSnapshot{
			ID:                    lease.ID,
			Kind:                  "leased",
			ModelName:             lease.holder.ModelName,
			TokenID:               lease.holder.TokenID,
			CompanyID:             lease.holder.CompanyID,
			DepartmentID:          lease.holder.DepartmentID,
			ThresholdTokens:       lease.ThresholdTokens,
			EstimatedPromptTokens: lease.holder.EstimatedPromptTokens,
			Priority:              lease.holder.Priority,
			Status:                status,
			CreatedAt:             lease.CreatedAt.Unix(),
			WaitSeconds:           secondsSince(lease.CreatedAt, now),
			RemainingTurns:        lease.RemainingTurns,
			LeaseTurns:            lease.LeaseTurns,
			IdleTimeoutSeconds:    lease.IdleTimeoutSeconds,
			IdleExpiresAt:         idleExpiresAt,
			Active:                lease.active,
		})
	}
	s.leaseMu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		if items[i].ModelName != items[j].ModelName {
			return items[i].ModelName < items[j].ModelName
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].CreatedAt < items[j].CreatedAt
	})

	enrichLongContextTaskOrganizationNames(items)

	return LongContextTasksSnapshot{
		Items: items,
		Total: len(items),
	}
}

func enrichLongContextTaskOrganizationNames(items []LongContextTaskSnapshot) {
	if len(items) == 0 || model.DB == nil {
		return
	}

	companyIDSet := make(map[int64]struct{})
	departmentIDSet := make(map[int64]struct{})
	for _, item := range items {
		if item.CompanyID > 0 {
			companyIDSet[item.CompanyID] = struct{}{}
		}
		if item.DepartmentID > 0 {
			departmentIDSet[item.DepartmentID] = struct{}{}
		}
	}

	companyNames := make(map[int64]string, len(companyIDSet))
	if len(companyIDSet) > 0 {
		companyIDs := make([]int64, 0, len(companyIDSet))
		for id := range companyIDSet {
			companyIDs = append(companyIDs, id)
		}

		var companies []model.Company
		if err := model.DB.Model(&model.Company{}).
			Select("id", "name").
			Where("id IN ?", companyIDs).
			Find(&companies).Error; err != nil {
			common.SysLog("failed to load queue task company names: " + err.Error())
		} else {
			for _, company := range companies {
				companyNames[company.Id] = company.Name
			}
		}
	}

	departmentNames := make(map[int64]string, len(departmentIDSet))
	if len(departmentIDSet) > 0 {
		departmentIDs := make([]int64, 0, len(departmentIDSet))
		for id := range departmentIDSet {
			departmentIDs = append(departmentIDs, id)
		}

		var departments []model.Department
		if err := model.DB.Model(&model.Department{}).
			Select("id", "name").
			Where("id IN ?", departmentIDs).
			Find(&departments).Error; err != nil {
			common.SysLog("failed to load queue task department names: " + err.Error())
		} else {
			for _, department := range departments {
				departmentNames[department.Id] = department.Name
			}
		}
	}

	for index := range items {
		items[index].CompanyName = companyNames[items[index].CompanyID]
		items[index].DepartmentName = departmentNames[items[index].DepartmentID]
	}
}

func (s *RequestQueueService) ensureLongContextLeaseMapLocked() {
	if s.longContextLeases == nil {
		s.longContextLeases = make(map[string]*LongContextLeaseUse)
	}
	if s.longContextOwners == nil {
		s.longContextOwners = make(map[string]map[string]struct{})
	}
}

func (s *RequestQueueService) addLongContextLeaseOwnerLocked(lease *LongContextLeaseUse) {
	if lease == nil || lease.OwnerKey == "" || lease.ID == "" {
		return
	}
	s.ensureLongContextLeaseMapLocked()
	if s.longContextOwners[lease.OwnerKey] == nil {
		s.longContextOwners[lease.OwnerKey] = make(map[string]struct{})
	}
	s.longContextOwners[lease.OwnerKey][lease.ID] = struct{}{}
}

func (s *RequestQueueService) removeLongContextLeaseOwnerLocked(lease *LongContextLeaseUse) {
	if lease == nil || lease.OwnerKey == "" || lease.ID == "" || s.longContextOwners == nil {
		return
	}
	delete(s.longContextOwners[lease.OwnerKey], lease.ID)
	if len(s.longContextOwners[lease.OwnerKey]) == 0 {
		delete(s.longContextOwners, lease.OwnerKey)
	}
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
	return s.GetEffectiveQueueConfigAt(modelName, time.Now())
}

func (s *RequestQueueService) GetEffectiveQueueConfigAt(modelName string, currentTime time.Time) EffectiveQueueConfig {
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
	timeSlots := queueConfig.GetTimeSlots()
	if len(timeSlots) > 0 {
		slotIndex := types.QueueTimeSlotConfigs(timeSlots).MatchTimeSlot(currentTime)
		if slotIndex < 0 {
			effectiveConfig.Enabled = false
			effectiveConfig.MaxQueueSize = 0
			effectiveConfig.QueueTimeout = 0
			effectiveConfig.LongContextTiers = nil
			return effectiveConfig
		}
		slot := timeSlots[slotIndex]
		effectiveConfig.Enabled = slot.Enabled
		effectiveConfig.MaxQueueSize = slot.MaxQueueSize
		effectiveConfig.QueueTimeout = slot.QueueTimeout
		effectiveConfig.LongContextTiers = slot.LongContextTiers
		return effectiveConfig
	}
	effectiveConfig.Enabled = queueConfig.Enabled
	effectiveConfig.MaxQueueSize = queueConfig.MaxQueueSize
	effectiveConfig.QueueTimeout = queueConfig.QueueTimeout
	effectiveConfig.LongContextTiers = queueConfig.GetLongContextTiers()
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
		if status.Queued == 0 && !hasLongContextTierActivity(status.LongContextTiers) {
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
		effectiveConfig := s.GetEffectiveQueueConfig(modelName)
		return ModelQueueSnapshot{
			Queued:           0,
			AvgWaitSec:       0,
			MaxWaitSec:       0,
			ThroughputRPM:    0,
			MaxQueueSize:     effectiveConfig.MaxQueueSize,
			Enabled:          effectiveConfig.Enabled,
			Buckets:          emptyQueueBuckets(),
			LongContextTiers: emptyLongContextTierStatus(effectiveConfig.LongContextTiers),
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

func MatchLongContextTiers(promptTokens int, tiers []types.QueueLongContextTier) []types.QueueLongContextTier {
	if promptTokens <= 0 || len(tiers) == 0 {
		return nil
	}
	normalized, err := types.NormalizeQueueLongContextTiers(tiers)
	if err != nil {
		return nil
	}
	matched := make([]types.QueueLongContextTier, 0, len(normalized))
	for _, tier := range normalized {
		if promptTokens >= tier.ThresholdTokens {
			matched = append(matched, tier)
		}
	}
	return matched
}

func highestLongContextTier(tiers []types.QueueLongContextTier) (types.QueueLongContextTier, bool) {
	normalized, err := types.NormalizeQueueLongContextTiers(tiers)
	if err != nil || len(normalized) == 0 {
		return types.QueueLongContextTier{}, false
	}
	return normalized[len(normalized)-1], true
}

func coversLongContextTiers(held []types.QueueLongContextTier, requested []types.QueueLongContextTier) bool {
	if len(requested) == 0 {
		return false
	}
	heldNormalized, err := types.NormalizeQueueLongContextTiers(held)
	if err != nil || len(heldNormalized) == 0 {
		return false
	}
	requestedNormalized, err := types.NormalizeQueueLongContextTiers(requested)
	if err != nil || len(requestedNormalized) == 0 {
		return false
	}
	heldByThreshold := make(map[int]struct{}, len(heldNormalized))
	for _, tier := range heldNormalized {
		heldByThreshold[tier.ThresholdTokens] = struct{}{}
	}
	for _, tier := range requestedNormalized {
		if _, ok := heldByThreshold[tier.ThresholdTokens]; !ok {
			return false
		}
	}
	return true
}

func hasLongContextTierActivity(tiers []types.QueueLongContextTierStatus) bool {
	for _, tier := range tiers {
		if tier.Running > 0 || tier.Queued > 0 {
			return true
		}
	}
	return false
}

func emptyLongContextTierStatus(tiers []types.QueueLongContextTier) []types.QueueLongContextTierStatus {
	if len(tiers) == 0 {
		return nil
	}
	status := make([]types.QueueLongContextTierStatus, 0, len(tiers))
	for _, tier := range tiers {
		status = append(status, types.QueueLongContextTierStatus{
			ThresholdTokens:         tier.ThresholdTokens,
			MaxRunning:              tier.MaxRunning,
			LeaseTurns:              tier.LeaseTurns,
			LeaseIdleTimeoutSeconds: tier.LeaseIdleTimeoutSeconds,
		})
	}
	return status
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

func secondsSince(start time.Time, now time.Time) int {
	if start.IsZero() {
		return 0
	}
	seconds := int(now.Sub(start).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
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
