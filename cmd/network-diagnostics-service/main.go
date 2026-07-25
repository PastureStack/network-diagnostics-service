package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/PastureStack/network-diagnostics-service/internal/aggregation"
	"github.com/PastureStack/network-diagnostics-service/internal/service"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "serve" {
		return runService(args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("network-diagnostics-service", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print the build version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	config, err := aggregation.Decode(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "invalid aggregation request")
		return 1
	}
	plan, err := aggregation.BuildPlan(config)
	if err != nil {
		fmt.Fprintln(stderr, "aggregation request failed validation")
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		fmt.Fprintln(stderr, "unable to encode plan")
		return 1
	}
	return 0
}

func runService(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("network-diagnostics-service serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("address", envOrDefault("PASTURESTACK_DIAGNOSTICS_ADDRESS", ":8080"), "HTTP listener address")
	dataDir := flags.String("data-dir", envOrDefault("PASTURESTACK_DIAGNOSTICS_DATA_DIR", "/var/lib/pasturestack-network-diagnostics"), "persistent data directory")
	historyLength := flags.Int("history-length", envIntOrDefault("PASTURESTACK_DIAGNOSTICS_HISTORY_LENGTH", 12), "snapshots retained per agent")
	maxAgents := flags.Int("max-agents", envIntOrDefault("PASTURESTACK_DIAGNOSTICS_MAX_AGENTS", 256), "maximum distinct agents")
	retentionHours := flags.Int("retention-hours", envIntOrDefault("PASTURESTACK_DIAGNOSTICS_RETENTION_HOURS", 24), "snapshot and bundle retention in hours")
	showVersion := flags.Bool("version", false, "print the build version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	config := service.Config{
		Address:       *address,
		Token:         os.Getenv("PASTURESTACK_DIAGNOSTICS_TOKEN"),
		DataDir:       *dataDir,
		HistoryLength: *historyLength,
		MaxAgents:     *maxAgents,
		Retention:     time.Duration(*retentionHours) * time.Hour,
		Version:       version,
	}
	if err := config.Validate(); err != nil {
		fmt.Fprintln(stderr, "invalid runtime configuration")
		return 1
	}
	runtime, err := service.New(config)
	if err != nil {
		fmt.Fprintln(stderr, "unable to initialize diagnostics storage")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintln(stdout, "network diagnostics service ready")
	if err := runtime.Serve(ctx); err != nil {
		fmt.Fprintln(stderr, "diagnostics service stopped unexpectedly")
		return 1
	}
	return 0
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
