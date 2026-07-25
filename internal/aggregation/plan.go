package aggregation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func BuildPlan(config Config) (Plan, error) {
	normalized, err := normalize(config)
	if err != nil {
		return Plan{}, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return Plan{}, fmt.Errorf("encode normalized aggregation request: %w", err)
	}
	digest := sha256.Sum256(encoded)

	summary := BundleSummary{Total: len(normalized.Bundles)}
	for _, bundle := range normalized.Bundles {
		summary.TotalBytes += bundle.Bytes
		if bundle.HighSensitivity {
			summary.HighSensitivity++
		}
	}
	return Plan{
		Schema:           PlanSchema,
		NormalizedSHA256: hex.EncodeToString(digest[:]),
		Authorization:    normalized.Authorization,
		Limits:           normalized.Limits,
		Bundles:          summary,
		Safeguards: Safeguards{
			Mode: "audit-only", OpensNetworkListener: false, ContactsAgents: false, ExecutesCommands: false,
			ReadsOrWritesBundles: false, IssuesDownloadGrants: false, AcceptsTargetIdentifiers: false,
			AuthenticatedOperatorVerified: false, SensitiveDataApprovalVerified: false, ManifestSignaturesVerified: false,
			AuthenticatedEncryptionVerified: false, DownloadAuditImplemented: false, SingleUseDownloadImplemented: false,
			AllowsDirectoryListing: false, ProductionReady: false,
		},
	}, nil
}
