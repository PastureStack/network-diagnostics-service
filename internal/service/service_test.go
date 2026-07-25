package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "0123456789abcdef0123456789abcdef"

func TestSnapshotBundleLifecycle(t *testing.T) {
	config := Config{
		Address:       "127.0.0.1:0",
		Token:         testToken,
		DataDir:       t.TempDir(),
		HistoryLength: 2,
		MaxAgents:     4,
		Retention:     24 * time.Hour,
		Version:       "v0.2.0",
	}
	server, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	snapshot := Snapshot{
		Schema:     snapshotSchema,
		AgentID:    strings.Repeat("a", 64),
		CapturedAt: time.Now().UTC().Truncate(time.Second),
		Version:    "v0.2.0",
		Metrics: Metrics{
			InterfaceCount:  2,
			ReceiveBytes:    100,
			TransmitBytes:   200,
			RouteCount:      3,
			DefaultRoutes:   1,
			NameServerCount: 2,
			UptimeSeconds:   300,
		},
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	response := authorizedRequest(t, http.MethodPost, httpServer.URL+"/v1/snapshots", payload)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("snapshot status=%d", response.StatusCode)
	}
	_ = response.Body.Close()

	response = authorizedRequest(t, http.MethodGet, httpServer.URL+"/v1/summary", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("summary status=%d", response.StatusCode)
	}
	var summary summaryResponse
	if err := json.NewDecoder(response.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if summary.AgentCount != 1 || summary.SnapshotCount != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	response = authorizedRequest(t, http.MethodPost, httpServer.URL+"/v1/bundles", nil)
	if response.StatusCode != http.StatusCreated {
		content, _ := io.ReadAll(response.Body)
		t.Fatalf("bundle status=%d body=%s", response.StatusCode, content)
	}
	var record bundleRecord
	if err := json.NewDecoder(response.Body).Decode(&record); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if !bundleIDPattern.MatchString(record.ID) || record.SnapshotCount != 1 {
		t.Fatalf("unexpected bundle record: %+v", record)
	}

	response = authorizedRequest(t, http.MethodGet, httpServer.URL+"/v1/bundles/"+record.ID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("download status=%d", response.StatusCode)
	}
	archiveBytes, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 || archive.File[0].Name != "manifest.json" {
		t.Fatalf("unexpected archive entries: %+v", archive.File)
	}

	response = authorizedRequest(t, http.MethodDelete, httpServer.URL+"/v1/bundles/"+record.ID, nil)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d", response.StatusCode)
	}
	_ = response.Body.Close()
}

func TestAuthenticationAndStrictInput(t *testing.T) {
	config := Config{
		Address:       "127.0.0.1:0",
		Token:         testToken,
		DataDir:       t.TempDir(),
		HistoryLength: 2,
		MaxAgents:     4,
		Retention:     time.Hour,
		Version:       "test",
	}
	server, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	response, err := http.Get(httpServer.URL + "/v1/summary")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.StatusCode)
	}
	_ = response.Body.Close()

	response = authorizedRequest(t, http.MethodPost, httpServer.URL+"/v1/snapshots", []byte(`{"schema":"pasturestack.network-diagnostics-snapshot/v1","unknown":true}`))
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid snapshot status=%d", response.StatusCode)
	}
	_ = response.Body.Close()
}

func TestTraditionalChineseHomePage(t *testing.T) {
	config := Config{
		Address:       "127.0.0.1:0",
		Token:         testToken,
		DataDir:       t.TempDir(),
		HistoryLength: 2,
		MaxAgents:     4,
		Retention:     time.Hour,
		Version:       "test",
	}
	server, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "zh-TW")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "網路診斷") {
		t.Fatalf("unexpected Traditional Chinese response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func authorizedRequest(t *testing.T, method, url string, payload []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+testToken)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
