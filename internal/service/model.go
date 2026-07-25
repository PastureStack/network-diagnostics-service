package service

import (
	"errors"
	"regexp"
	"time"
	"unicode"
)

const snapshotSchema = "pasturestack.network-diagnostics-snapshot/v1"

var (
	agentIDPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	bundleIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	versionPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
)

type Config struct {
	Address       string
	Token         string
	DataDir       string
	HistoryLength int
	MaxAgents     int
	Retention     time.Duration
	Version       string
}

type Snapshot struct {
	Schema     string    `json:"schema"`
	AgentID    string    `json:"agent_id"`
	CapturedAt time.Time `json:"captured_at"`
	Version    string    `json:"version"`
	Metrics    Metrics   `json:"metrics"`
}

type Metrics struct {
	InterfaceCount   uint64 `json:"interface_count"`
	ReceiveBytes     uint64 `json:"receive_bytes"`
	ReceiveErrors    uint64 `json:"receive_errors"`
	ReceiveDrops     uint64 `json:"receive_drops"`
	TransmitBytes    uint64 `json:"transmit_bytes"`
	TransmitErrors   uint64 `json:"transmit_errors"`
	TransmitDrops    uint64 `json:"transmit_drops"`
	RouteCount       uint64 `json:"route_count"`
	DefaultRoutes    uint64 `json:"default_route_count"`
	NameServerCount  uint64 `json:"name_server_count"`
	SearchListCount  uint64 `json:"search_list_count"`
	UptimeSeconds    uint64 `json:"uptime_seconds"`
	CollectionErrors uint64 `json:"collection_error_count"`
}

type summaryResponse struct {
	Schema         string    `json:"schema"`
	AgentCount     int       `json:"agent_count"`
	SnapshotCount  int       `json:"snapshot_count"`
	LatestSnapshot time.Time `json:"latest_snapshot,omitempty"`
}

type bundleRecord struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	SizeBytes     int64     `json:"size_bytes"`
	SnapshotCount int       `json:"snapshot_count"`
}

type bundleList struct {
	Data []bundleRecord `json:"data"`
}

type storedSnapshot struct {
	Snapshot
	path string
}

func (c Config) Validate() error {
	if c.Address == "" || c.DataDir == "" {
		return errors.New("listener address and data directory are required")
	}
	if !validToken(c.Token) {
		return errors.New("token must contain 32 through 256 printable non-space ASCII characters")
	}
	if c.HistoryLength < 1 || c.HistoryLength > 100 {
		return errors.New("history length is outside the supported range")
	}
	if c.MaxAgents < 1 || c.MaxAgents > 1024 {
		return errors.New("maximum agent count is outside the supported range")
	}
	if c.Retention < time.Hour || c.Retention > 7*24*time.Hour {
		return errors.New("retention is outside the supported range")
	}
	if !versionPattern.MatchString(c.Version) {
		return errors.New("build version is invalid")
	}
	return nil
}

func validToken(token string) bool {
	if len(token) < 32 || len(token) > 256 {
		return false
	}
	for _, character := range token {
		if character > unicode.MaxASCII || character <= ' ' || character == 0x7f {
			return false
		}
	}
	return true
}

func validateSnapshot(snapshot Snapshot, now time.Time) error {
	if snapshot.Schema != snapshotSchema || !agentIDPattern.MatchString(snapshot.AgentID) || !versionPattern.MatchString(snapshot.Version) {
		return errors.New("snapshot identity or schema is invalid")
	}
	if snapshot.CapturedAt.IsZero() || snapshot.CapturedAt.Before(now.Add(-7*24*time.Hour)) || snapshot.CapturedAt.After(now.Add(10*time.Minute)) {
		return errors.New("snapshot timestamp is outside the accepted window")
	}
	const maxCounter = uint64(1<<63 - 1)
	values := []uint64{
		snapshot.Metrics.InterfaceCount,
		snapshot.Metrics.ReceiveBytes,
		snapshot.Metrics.ReceiveErrors,
		snapshot.Metrics.ReceiveDrops,
		snapshot.Metrics.TransmitBytes,
		snapshot.Metrics.TransmitErrors,
		snapshot.Metrics.TransmitDrops,
		snapshot.Metrics.RouteCount,
		snapshot.Metrics.DefaultRoutes,
		snapshot.Metrics.NameServerCount,
		snapshot.Metrics.SearchListCount,
		snapshot.Metrics.UptimeSeconds,
		snapshot.Metrics.CollectionErrors,
	}
	for _, value := range values {
		if value > maxCounter {
			return errors.New("snapshot counter is outside the accepted range")
		}
	}
	if snapshot.Metrics.InterfaceCount > 65535 ||
		snapshot.Metrics.RouteCount > 1_000_000 ||
		snapshot.Metrics.DefaultRoutes > snapshot.Metrics.RouteCount ||
		snapshot.Metrics.NameServerCount > 1024 ||
		snapshot.Metrics.SearchListCount > 4096 ||
		snapshot.Metrics.CollectionErrors > 16 {
		return errors.New("snapshot metric is outside the accepted range")
	}
	return nil
}
