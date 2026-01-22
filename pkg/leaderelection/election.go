package leaderelection

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

// Config contains the configuration for leader election
type Config struct {
	// Namespace where the lease object will be created
	Namespace string

	// LeaseName is the name of the lease object
	LeaseName string

	// Identity is the unique identifier for this instance (typically pod name)
	Identity string

	// LeaseDuration is the duration that non-leader candidates will wait to force acquire leadership
	LeaseDuration time.Duration

	// RenewDeadline is the duration that the acting leader will retry refreshing leadership before giving up
	RenewDeadline time.Duration

	// RetryPeriod is the duration the LeaderElector clients should wait between tries of actions
	RetryPeriod time.Duration
}

// LeaderElector manages leader election for high availability
type LeaderElector struct {
	config         *Config
	client         kubernetes.Interface
	leaderElection *leaderelection.LeaderElector
	isLeader       atomic.Bool
	currentLeader  atomic.Value

	// Callbacks
	onStartedLeading func(ctx context.Context)
	onStoppedLeading func()
	onNewLeader      func(identity string)
}

// NewLeaderElector creates a new LeaderElector instance
func NewLeaderElector(cfg *Config, client kubernetes.Interface) (*LeaderElector, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if cfg.Identity == "" {
		// Try to get pod name from environment
		cfg.Identity = os.Getenv("POD_NAME")
		if cfg.Identity == "" {
			// Use hostname as fallback
			hostname, err := os.Hostname()
			if err != nil {
				return nil, fmt.Errorf("failed to determine identity: %w", err)
			}
			cfg.Identity = hostname
		}
	}

	klog.InfoS("Creating leader elector",
		"namespace", cfg.Namespace,
		"leaseName", cfg.LeaseName,
		"identity", cfg.Identity)

	return &LeaderElector{
		config: cfg,
		client: client,
	}, nil
}

// SetCallbacks sets the callback functions for leader election events
func (le *LeaderElector) SetCallbacks(
	onStartedLeading func(ctx context.Context),
	onStoppedLeading func(),
	onNewLeader func(identity string),
) {
	le.onStartedLeading = onStartedLeading
	le.onStoppedLeading = onStoppedLeading
	le.onNewLeader = onNewLeader
}

// Run starts the leader election process and blocks until context is cancelled
func (le *LeaderElector) Run(ctx context.Context) error {
	// Create the resource lock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      le.config.LeaseName,
			Namespace: le.config.Namespace,
		},
		Client: le.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: le.config.Identity,
		},
	}

	// Create leader election config
	leConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   le.config.LeaseDuration,
		RenewDeadline:   le.config.RenewDeadline,
		RetryPeriod:     le.config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				le.isLeader.Store(true)
				klog.InfoS("Started leading", "identity", le.config.Identity)
				if le.onStartedLeading != nil {
					le.onStartedLeading(ctx)
				}
			},
			OnStoppedLeading: func() {
				le.isLeader.Store(false)
				klog.InfoS("Stopped leading", "identity", le.config.Identity)
				if le.onStoppedLeading != nil {
					le.onStoppedLeading()
				}
			},
			OnNewLeader: func(identity string) {
				le.currentLeader.Store(identity)
				if identity == le.config.Identity {
					klog.InfoS("Successfully acquired leadership", "identity", identity)
				} else {
					klog.InfoS("New leader elected", "leader", identity)
				}
				if le.onNewLeader != nil {
					le.onNewLeader(identity)
				}
			},
		},
	}

	// Create and run leader elector
	elector, err := leaderelection.NewLeaderElector(leConfig)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	le.leaderElection = elector

	klog.InfoS("Starting leader election",
		"leaseDuration", le.config.LeaseDuration,
		"renewDeadline", le.config.RenewDeadline,
		"retryPeriod", le.config.RetryPeriod)

	// Run the leader election (blocks until context is cancelled)
	elector.Run(ctx)

	return nil
}

// IsLeader returns true if this instance is currently the leader
func (le *LeaderElector) IsLeader() bool {
	return le.isLeader.Load()
}

// GetLeader returns the identity of the current leader
func (le *LeaderElector) GetLeader() string {
	if leader := le.currentLeader.Load(); leader != nil {
		return leader.(string)
	}
	return ""
}

// GetIdentity returns the identity of this instance
func (le *LeaderElector) GetIdentity() string {
	return le.config.Identity
}
