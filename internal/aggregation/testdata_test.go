package aggregation

import "strings"

func digest(value string) string { return strings.Repeat(value, 64) }

func validConfig() Config {
	return Config{
		Schema:               InputSchema,
		CollectionPlanSHA256: digest("1"),
		Authorization:        Authorization{OperatorAuthenticated: true, SensitiveDataApproved: true},
		Limits: Limits{
			MaxAgentBundles: 32, MaxBundleBytes: 16 << 20, MaxTotalBytes: 128 << 20,
			JobTimeoutSeconds: 600, RetentionSeconds: 3600, DownloadTTLSeconds: 300,
		},
		Bundles: []Bundle{
			{ManifestSHA256: digest("2"), ContentSHA256: digest("3"), Bytes: 1 << 20, ProbeCount: 4, HighSensitivity: true, RedactionStatus: "passed", Encrypted: true},
			{ManifestSHA256: digest("4"), ContentSHA256: digest("5"), Bytes: 2 << 20, ProbeCount: 2, RedactionStatus: "passed", Encrypted: true},
		},
	}
}
