package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const validDocument = `{
  "schema":"pasturestack.diagnostics-aggregation/v1",
  "collection_plan_sha256":"1111111111111111111111111111111111111111111111111111111111111111",
  "authorization":{"operator_authenticated":true,"sensitive_data_approved":true},
  "limits":{"max_agent_bundles":32,"max_bundle_bytes":16777216,"max_total_bytes":134217728,"job_timeout_seconds":600,"retention_seconds":3600,"download_ttl_seconds":300},
  "bundles":[{"manifest_sha256":"2222222222222222222222222222222222222222222222222222222222222222","content_sha256":"3333333333333333333333333333333333333333333333333333333333333333","bytes":1048576,"probe_count":4,"high_sensitivity":true,"redaction_status":"passed","encrypted":true}]
}`

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunFromStandardInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, strings.NewReader(validDocument), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema": "pasturestack.diagnostics-aggregation-plan/v1"`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestRunVersionAndArguments(t *testing.T) {
	oldVersion := version
	version = "test-version"
	t.Cleanup(func() { version = oldVersion })
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 || strings.TrimSpace(stdout.String()) != "test-version" {
		t.Fatalf("code=%d output=%q", code, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"extra"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("positional code=%d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--unknown"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("flag code=%d", code)
	}
}

func TestRunErrors(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		input string
	}{
		{name: "decode", input: "{"},
		{name: "validation", input: `{"schema":"pasturestack.diagnostics-aggregation/v1"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, strings.NewReader(test.input), &stdout, &stderr); code != 1 {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
		})
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--file", "request.json"}, strings.NewReader(validDocument), &stdout, &stderr); code != 2 {
		t.Fatalf("removed file flag code=%d", code)
	}
	stderr.Reset()
	if code := run(nil, strings.NewReader(validDocument), errorWriter{}, &stderr); code != 1 {
		t.Fatalf("writer code=%d", code)
	}
}
