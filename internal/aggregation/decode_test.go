package aggregation

import (
	"errors"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestDecodeStrictJSON(t *testing.T) {
	t.Parallel()
	valid := `{"schema":"pasturestack.diagnostics-aggregation/v1","collection_plan_sha256":"` + digest("1") + `","authorization":{"operator_authenticated":true,"sensitive_data_approved":false},"limits":{"max_agent_bundles":1,"max_bundle_bytes":65536,"max_total_bytes":65536,"job_timeout_seconds":60,"retention_seconds":300,"download_ttl_seconds":60},"bundles":[]}`
	if _, err := Decode(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"schema":"one","schema":"two"}`,
		`{"unknown":true}`,
		valid + valid,
		`{"schema":`,
		`{"limits":{"max_bundle_bytes":1,"max_bundle_bytes":2}}`,
	} {
		if _, err := Decode(strings.NewReader(input)); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
	if _, err := Decode(strings.NewReader(strings.Repeat(" ", MaxInputBytes+1))); err == nil {
		t.Fatal("expected size error")
	}
	if _, err := Decode(failingReader{}); err == nil {
		t.Fatal("expected read error")
	}
}
