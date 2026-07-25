package service

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxBundleBytes = 64 << 20

type Store struct {
	sync.RWMutex
	config    Config
	snapshots map[string][]storedSnapshot
}

func newStore(config Config) (*Store, error) {
	store := &Store{
		config:    config,
		snapshots: map[string][]storedSnapshot{},
	}
	for _, path := range []string{store.snapshotDir(), store.bundleDir()} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return nil, err
		}
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) snapshotDir() string {
	return filepath.Join(s.config.DataDir, "snapshots")
}

func (s *Store) bundleDir() string {
	return filepath.Join(s.config.DataDir, "bundles")
}

func (s *Store) load() error {
	entries, err := os.ReadDir(s.snapshotDir())
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.snapshotDir(), entry.Name())
		content, readErr := os.ReadFile(path)
		if readErr != nil || len(content) > 64<<10 {
			continue
		}
		var snapshot Snapshot
		if json.Unmarshal(content, &snapshot) != nil || validateSnapshot(snapshot, now) != nil {
			continue
		}
		if snapshot.CapturedAt.Before(now.Add(-s.config.Retention)) {
			_ = os.Remove(path)
			continue
		}
		s.snapshots[snapshot.AgentID] = append(s.snapshots[snapshot.AgentID], storedSnapshot{
			Snapshot: snapshot,
			path:     path,
		})
	}
	s.pruneLocked(now)
	return nil
}

func (s *Store) Add(snapshot Snapshot) error {
	s.Lock()
	defer s.Unlock()

	now := time.Now().UTC()
	if err := validateSnapshot(snapshot, now); err != nil {
		return err
	}
	if _, exists := s.snapshots[snapshot.AgentID]; !exists && len(s.snapshots) >= s.config.MaxAgents {
		return errors.New("agent limit reached")
	}
	randomID, err := randomHex(8)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%020d-%s.json", now.UnixNano(), randomID)
	path := filepath.Join(s.snapshotDir(), name)
	if err := writeJSONAtomic(path, snapshot); err != nil {
		return err
	}
	s.snapshots[snapshot.AgentID] = append(s.snapshots[snapshot.AgentID], storedSnapshot{
		Snapshot: snapshot,
		path:     path,
	})
	s.pruneLocked(now)
	return nil
}

func (s *Store) Summary() summaryResponse {
	s.RLock()
	defer s.RUnlock()

	response := summaryResponse{
		Schema:     "pasturestack.network-diagnostics-summary/v1",
		AgentCount: len(s.snapshots),
	}
	for _, snapshots := range s.snapshots {
		response.SnapshotCount += len(snapshots)
		for _, snapshot := range snapshots {
			if snapshot.CapturedAt.After(response.LatestSnapshot) {
				response.LatestSnapshot = snapshot.CapturedAt
			}
		}
	}
	return response
}

func (s *Store) CreateBundle() (bundleRecord, error) {
	s.Lock()
	defer s.Unlock()

	now := time.Now().UTC()
	s.pruneLocked(now)
	var snapshots []Snapshot
	for _, agentSnapshots := range s.snapshots {
		for _, snapshot := range agentSnapshots {
			snapshots = append(snapshots, snapshot.Snapshot)
		}
	}
	if len(snapshots) == 0 {
		return bundleRecord{}, errors.New("no snapshots are available")
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].AgentID == snapshots[j].AgentID {
			return snapshots[i].CapturedAt.Before(snapshots[j].CapturedAt)
		}
		return snapshots[i].AgentID < snapshots[j].AgentID
	})

	id, err := randomHex(16)
	if err != nil {
		return bundleRecord{}, err
	}
	record := bundleRecord{
		ID:            id,
		CreatedAt:     now,
		SnapshotCount: len(snapshots),
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest := map[string]any{
		"schema":          "pasturestack.network-diagnostics-bundle/v1",
		"created_at":      now,
		"snapshot_count":  len(snapshots),
		"service_version": s.config.Version,
	}
	if err := writeZipJSON(writer, "manifest.json", manifest); err != nil {
		return bundleRecord{}, err
	}
	for index, snapshot := range snapshots {
		name := fmt.Sprintf("snapshots/%06d.json", index+1)
		if err := writeZipJSON(writer, name, snapshot); err != nil {
			return bundleRecord{}, err
		}
		if buffer.Len() > maxBundleBytes {
			return bundleRecord{}, errors.New("bundle exceeds the size limit")
		}
	}
	if err := writer.Close(); err != nil {
		return bundleRecord{}, err
	}
	if buffer.Len() > maxBundleBytes {
		return bundleRecord{}, errors.New("bundle exceeds the size limit")
	}
	record.SizeBytes = int64(buffer.Len())
	if err := writeBytesAtomic(filepath.Join(s.bundleDir(), id+".zip"), buffer.Bytes()); err != nil {
		return bundleRecord{}, err
	}
	s.pruneBundlesLocked(now)
	return record, nil
}

func (s *Store) ListBundles() ([]bundleRecord, error) {
	s.Lock()
	defer s.Unlock()
	s.pruneBundlesLocked(time.Now().UTC())
	return s.listBundlesLocked()
}

func (s *Store) BundlePath(id string) (string, error) {
	if !bundleIDPattern.MatchString(id) {
		return "", errors.New("bundle identifier is invalid")
	}
	path := filepath.Join(s.bundleDir(), id+".zip")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", os.ErrNotExist
	}
	return path, nil
}

func (s *Store) DeleteBundle(id string) error {
	if !bundleIDPattern.MatchString(id) {
		return errors.New("bundle identifier is invalid")
	}
	return os.Remove(filepath.Join(s.bundleDir(), id+".zip"))
}

func (s *Store) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.config.Retention)
	for agentID, snapshots := range s.snapshots {
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].CapturedAt.Before(snapshots[j].CapturedAt)
		})
		keepFrom := 0
		for keepFrom < len(snapshots) && snapshots[keepFrom].CapturedAt.Before(cutoff) {
			_ = os.Remove(snapshots[keepFrom].path)
			keepFrom++
		}
		snapshots = snapshots[keepFrom:]
		if len(snapshots) > s.config.HistoryLength {
			removeCount := len(snapshots) - s.config.HistoryLength
			for _, snapshot := range snapshots[:removeCount] {
				_ = os.Remove(snapshot.path)
			}
			snapshots = snapshots[removeCount:]
		}
		if len(snapshots) == 0 {
			delete(s.snapshots, agentID)
		} else {
			s.snapshots[agentID] = snapshots
		}
	}
}

func (s *Store) pruneBundlesLocked(now time.Time) {
	entries, err := os.ReadDir(s.bundleDir())
	if err != nil {
		return
	}
	cutoff := now.Add(-s.config.Retention)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		path := filepath.Join(s.bundleDir(), entry.Name())
		info, statErr := entry.Info()
		if statErr == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}

func (s *Store) listBundlesLocked() ([]bundleRecord, error) {
	entries, err := os.ReadDir(s.bundleDir())
	if err != nil {
		return nil, err
	}
	records := make([]bundleRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".zip")
		if !bundleIDPattern.MatchString(id) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		records = append(records, bundleRecord{
			ID:        id,
			CreatedAt: info.ModTime().UTC(),
			SizeBytes: info.Size(),
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

func writeZipJSON(writer *zip.Writer, name string, value any) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.Modified = time.Unix(0, 0).UTC()
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetEscapeHTML(true)
	return encoder.Encode(value)
}

func writeJSONAtomic(path string, value any) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	return writeBytesAtomic(path, buffer.Bytes())
}

func writeBytesAtomic(path string, content []byte) error {
	randomID, err := randomHex(8)
	if err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+randomID+".tmp")
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := io.Copy(file, bytes.NewReader(content)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func randomHex(bytesLength int) (string, error) {
	content := make([]byte, bytesLength)
	if _, err := rand.Read(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil
}
