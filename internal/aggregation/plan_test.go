package aggregation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPlanSummaryAndPrivacy(t *testing.T) {
	t.Parallel()
	config := validConfig()
	plan, err := BuildPlan(config)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Schema != PlanSchema || len(plan.NormalizedSHA256) != 64 {
		t.Fatalf("unexpected header: %#v", plan)
	}
	want := BundleSummary{Total: 2, HighSensitivity: 1, TotalBytes: 3 << 20}
	if plan.Bundles != want {
		t.Fatalf("summary=%#v want=%#v", plan.Bundles, want)
	}
	s := plan.Safeguards
	if s.Mode != "audit-only" || s.OpensNetworkListener || s.ContactsAgents || s.ExecutesCommands || s.ReadsOrWritesBundles || s.IssuesDownloadGrants || s.AcceptsTargetIdentifiers || s.AuthenticatedOperatorVerified || s.SensitiveDataApprovalVerified || s.ManifestSignaturesVerified || s.AuthenticatedEncryptionVerified || s.DownloadAuditImplemented || s.SingleUseDownloadImplemented || s.AllowsDirectoryListing || s.ProductionReady {
		t.Fatalf("unexpected safeguards: %#v", s)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{config.CollectionPlanSHA256, config.Bundles[0].ManifestSHA256, config.Bundles[0].ContentSHA256} {
		if strings.Contains(string(encoded), value) {
			t.Fatalf("output leaked an input digest")
		}
	}
}

func TestBuildPlanDeterministic(t *testing.T) {
	t.Parallel()
	first := validConfig()
	second := validConfig()
	second.Bundles[0], second.Bundles[1] = second.Bundles[1], second.Bundles[0]
	left, err := BuildPlan(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := BuildPlan(second)
	if err != nil {
		t.Fatal(err)
	}
	if left.NormalizedSHA256 != right.NormalizedSHA256 {
		t.Fatalf("digests differ: %s != %s", left.NormalizedSHA256, right.NormalizedSHA256)
	}
}
