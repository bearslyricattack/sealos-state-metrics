package pod

import (
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// AggregatedPod represents aggregated pod information
type AggregatedPod struct {
	Namespace  string
	Phase      corev1.PodPhase
	Count      int
	LastSeen   time.Time
	Pods       map[string]bool // pod names
}

// PodAggregator aggregates similar pods to reduce metric cardinality
type PodAggregator struct {
	mu           sync.RWMutex
	windowSize   time.Duration
	aggregated   map[string]*AggregatedPod // key: namespace/phase
	abnormalPods map[string]time.Time      // key: namespace/pod, value: first seen time
	stopCh       chan struct{}
}

// NewPodAggregator creates a new pod aggregator
func NewPodAggregator(windowSize time.Duration) *PodAggregator {
	return &PodAggregator{
		windowSize:   windowSize,
		aggregated:   make(map[string]*AggregatedPod),
		abnormalPods: make(map[string]time.Time),
		stopCh:       make(chan struct{}),
	}
}

// Start starts the aggregator cleanup goroutine
func (a *PodAggregator) Start() {
	go a.cleanupLoop()
	klog.V(4).InfoS("Pod aggregator started", "windowSize", a.windowSize)
}

// Stop stops the aggregator
func (a *PodAggregator) Stop() {
	close(a.stopCh)
	klog.V(4).Info("Pod aggregator stopped")
}

// AddPod adds a pod to the aggregator
func (a *PodAggregator) AddPod(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	key := podAggregationKey(pod)
	now := time.Now()

	if agg, exists := a.aggregated[key]; exists {
		agg.Count = len(agg.Pods) + 1
		agg.LastSeen = now
		agg.Pods[pod.Name] = true
	} else {
		a.aggregated[key] = &AggregatedPod{
			Namespace: pod.Namespace,
			Phase:     pod.Status.Phase,
			Count:     1,
			LastSeen:  now,
			Pods:      map[string]bool{pod.Name: true},
		}
	}

	// Track abnormal pods
	if isPodAbnormal(pod) {
		abnormalKey := pod.Namespace + "/" + pod.Name
		if _, exists := a.abnormalPods[abnormalKey]; !exists {
			a.abnormalPods[abnormalKey] = now
		}
	}
}

// RemovePod removes a pod from the aggregator
func (a *PodAggregator) RemovePod(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	key := podAggregationKey(pod)
	if agg, exists := a.aggregated[key]; exists {
		delete(agg.Pods, pod.Name)
		agg.Count = len(agg.Pods)
		if agg.Count == 0 {
			delete(a.aggregated, key)
		}
	}

	// Remove from abnormal tracking
	abnormalKey := pod.Namespace + "/" + pod.Name
	delete(a.abnormalPods, abnormalKey)
}

// GetAggregated returns all aggregated pods
func (a *PodAggregator) GetAggregated() map[string]*AggregatedPod {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]*AggregatedPod, len(a.aggregated))
	for k, v := range a.aggregated {
		result[k] = v
	}
	return result
}

// GetAbnormalDuration returns how long a pod has been abnormal
func (a *PodAggregator) GetAbnormalDuration(namespace, podName string) time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	key := namespace + "/" + podName
	if firstSeen, exists := a.abnormalPods[key]; exists {
		return time.Since(firstSeen)
	}
	return 0
}

// cleanupLoop periodically cleans up old entries
func (a *PodAggregator) cleanupLoop() {
	ticker := time.NewTicker(a.windowSize)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.cleanup()
		case <-a.stopCh:
			return
		}
	}
}

// cleanup removes old entries
func (a *PodAggregator) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	threshold := now.Add(-a.windowSize * 2) // Keep entries for 2 windows

	// Clean up aggregated entries
	for key, agg := range a.aggregated {
		if agg.LastSeen.Before(threshold) {
			delete(a.aggregated, key)
			klog.V(4).InfoS("Cleaned up aggregated pod entry", "key", key)
		}
	}
}

// podAggregationKey returns the aggregation key for a pod
func podAggregationKey(pod *corev1.Pod) string {
	return pod.Namespace + "/" + string(pod.Status.Phase)
}

// isPodAbnormal returns true if the pod is in an abnormal state
func isPodAbnormal(pod *corev1.Pod) bool {
	// Consider pod abnormal if:
	// - Phase is Failed or Unknown
	// - Phase is Pending for too long (handled elsewhere)
	// - Not ready for too long (handled elsewhere)
	switch pod.Status.Phase {
	case corev1.PodFailed, corev1.PodUnknown:
		return true
	case corev1.PodPending:
		// Check if it's stuck in pending
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
				return true
			}
		}
	case corev1.PodRunning:
		// Check if containers are not ready
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
				return true
			}
		}
	}
	return false
}
