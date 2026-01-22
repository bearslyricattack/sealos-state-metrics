package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zijiren233/sealos-state-metric/cmd/sealos-state-metric/app"
	"github.com/zijiren233/sealos-state-metric/pkg/config"
	"k8s.io/klog/v2"
)

func main() {
	// Parse command-line flags
	opts := app.NewOptions()
	opts.AddFlags(flag.CommandLine)

	// Add klog flags
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	// Load configuration
	loader := config.NewConfigLoader(opts.ConfigFile, opts.EnvFile)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Override config with command-line flags
	if opts.MetricsAddress != "" {
		cfg.Server.Address = opts.MetricsAddress
	}
	if opts.KubeconfigFile != "" {
		cfg.Kubernetes.Kubeconfig = opts.KubeconfigFile
	}
	if opts.EnabledCollectors != "" {
		cfg.EnabledCollectors = opts.ParsedEnabledCollectors()
	}
	cfg.LeaderElection.Enabled = opts.LeaderElectionMode

	// Override logging level
	if opts.LogLevel != "" {
		cfg.Logging.Level = opts.LogLevel
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	klog.InfoS("Starting Sealos State Metric",
		"version", app.Version,
		"gitCommit", app.GitCommit,
		"buildDate", app.BuildDate)

	klog.InfoS("Configuration loaded",
		"collectors", cfg.EnabledCollectors,
		"leaderElection", cfg.LeaderElection.Enabled,
		"metricsAddress", cfg.Server.Address)

	// Create and run server
	ctx := context.Background()
	server, err := app.NewServer(cfg, opts.ConfigFile)
	if err != nil {
		klog.ErrorS(err, "Failed to create server")
		os.Exit(1)
	}

	if err := server.Run(ctx); err != nil {
		klog.ErrorS(err, "Server exited with error")
		os.Exit(1)
	}

	klog.Info("Server exited successfully")
}
