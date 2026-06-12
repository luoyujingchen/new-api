package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var requestQueueSchedulerOnce sync.Once

func StartRequestQueueScheduler() {
	requestQueueSchedulerOnce.Do(func() {
		go GetRequestQueueService().runScheduler()
	})
}

func (s *RequestQueueService) runScheduler() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case modelName := <-s.retryEvents:
			s.dispatchModel(modelName)
		case <-ticker.C:
			s.dispatchAll()
		}
	}
}

func (s *RequestQueueService) dispatchAll() {
	for _, modelName := range s.modelNames() {
		s.dispatchModel(modelName)
	}
}

func (s *RequestQueueService) dispatchModel(modelName string) {
	queue := s.getModelQueue(modelName)
	if queue == nil || queue.Size() == 0 {
		return
	}

	for {
		if !s.tryDispatchOnce(queue) {
			return
		}
	}
}

func (s *RequestQueueService) tryDispatchOnce(queue *ModelQueue) bool {
	excludedBuckets := make(map[int]struct{}, 10)
	excludedRequests := make(map[*QueuedRequest]struct{})
	effectiveConfig := s.GetEffectiveQueueConfig(queue.modelName)
	for len(excludedBuckets) < 10 {
		candidate, bucketPriority, bucketFound := queue.AcquireCandidateByWeight(excludedBuckets, excludedRequests, effectiveConfig.LongContextTiers)
		if !bucketFound {
			return false
		}
		if candidate == nil {
			excludedBuckets[bucketPriority] = struct{}{}
			continue
		}
		allowed, err := candidate.canProceed()
		if err != nil {
			queue.ReleaseLongContextSlots(candidate)
			common.SysError("request queue rate-limit recheck failed: " + err.Error())
			return false
		}
		if !allowed {
			queue.ReleaseLongContextSlots(candidate)
			excludedRequests[candidate] = struct{}{}
			continue
		}
		if !queue.Dequeue(candidate) {
			queue.ReleaseLongContextSlots(candidate)
			continue
		}
		candidate.markReady()
		return true
	}
	return false
}
