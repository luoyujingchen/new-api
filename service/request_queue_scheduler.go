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
	excluded := make(map[int]struct{}, 10)
	for len(excluded) < 10 {
		candidate, bucketPriority := queue.PeekByWeight(excluded)
		if candidate == nil {
			return false
		}
		allowed, err := candidate.canProceed()
		if err != nil {
			common.SysError("request queue rate-limit recheck failed: " + err.Error())
			return false
		}
		if !allowed {
			excluded[bucketPriority] = struct{}{}
			continue
		}
		if !queue.DequeueIfHead(candidate) {
			continue
		}
		candidate.markReady()
		return true
	}
	return false
}