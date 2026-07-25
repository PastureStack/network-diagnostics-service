package aggregation

import "testing"

func TestValidationRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"schema", func(c *Config) { c.Schema = "wrong" }},
		{"collection digest length", func(c *Config) { c.CollectionPlanSHA256 = "short" }},
		{"collection digest format", func(c *Config) { c.CollectionPlanSHA256 = digest("z") }},
		{"operator", func(c *Config) { c.Authorization.OperatorAuthenticated = false }},
		{"max bundles low", func(c *Config) { c.Limits.MaxAgentBundles = 0 }},
		{"max bundles high", func(c *Config) { c.Limits.MaxAgentBundles = MaxBundles + 1 }},
		{"bundle bytes low", func(c *Config) { c.Limits.MaxBundleBytes = (64 << 10) - 1 }},
		{"bundle bytes high", func(c *Config) { c.Limits.MaxBundleBytes = (64 << 20) + 1 }},
		{"total below bundle", func(c *Config) { c.Limits.MaxTotalBytes = c.Limits.MaxBundleBytes - 1 }},
		{"total high", func(c *Config) { c.Limits.MaxTotalBytes = MaxTotalBytes + 1 }},
		{"job timeout low", func(c *Config) { c.Limits.JobTimeoutSeconds = 59 }},
		{"job timeout high", func(c *Config) { c.Limits.JobTimeoutSeconds = 1801 }},
		{"retention low", func(c *Config) { c.Limits.RetentionSeconds = 299 }},
		{"retention high", func(c *Config) { c.Limits.RetentionSeconds = 86401 }},
		{"download low", func(c *Config) { c.Limits.DownloadTTLSeconds = 59 }},
		{"download high", func(c *Config) { c.Limits.DownloadTTLSeconds = 3601 }},
		{"download above retention", func(c *Config) { c.Limits.RetentionSeconds = 300; c.Limits.DownloadTTLSeconds = 301 }},
		{"no bundles", func(c *Config) { c.Bundles = nil }},
		{"above configured bundles", func(c *Config) { c.Limits.MaxAgentBundles = 1 }},
		{"manifest digest", func(c *Config) { c.Bundles[0].ManifestSHA256 = "bad" }},
		{"content digest", func(c *Config) { c.Bundles[0].ContentSHA256 = "bad" }},
		{"duplicate manifest", func(c *Config) { c.Bundles[1].ManifestSHA256 = c.Bundles[0].ManifestSHA256 }},
		{"duplicate content", func(c *Config) { c.Bundles[1].ContentSHA256 = c.Bundles[0].ContentSHA256 }},
		{"empty bytes", func(c *Config) { c.Bundles[0].Bytes = 0 }},
		{"bundle too large", func(c *Config) { c.Bundles[0].Bytes = c.Limits.MaxBundleBytes + 1 }},
		{"probe low", func(c *Config) { c.Bundles[0].ProbeCount = 0 }},
		{"probe high", func(c *Config) { c.Bundles[0].ProbeCount = 33 }},
		{"redaction", func(c *Config) { c.Bundles[0].RedactionStatus = "pending" }},
		{"encryption", func(c *Config) { c.Bundles[0].Encrypted = false }},
		{"sensitive approval", func(c *Config) { c.Authorization.SensitiveDataApproved = false }},
		{"total bytes", func(c *Config) { c.Limits.MaxTotalBytes = 2 << 20 }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			config := validConfig()
			test.edit(&config)
			if _, err := BuildPlan(config); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestValidationBoundariesAndDigestNormalization(t *testing.T) {
	t.Parallel()
	config := validConfig()
	config.CollectionPlanSHA256 = digest("A")
	config.Limits = Limits{MaxAgentBundles: 1, MaxBundleBytes: 64 << 10, MaxTotalBytes: 64 << 10, JobTimeoutSeconds: 60, RetentionSeconds: 300, DownloadTTLSeconds: 60}
	config.Bundles = []Bundle{{ManifestSHA256: digest("B"), ContentSHA256: digest("C"), Bytes: 1, ProbeCount: 1, RedactionStatus: "passed", Encrypted: true}}
	if _, err := BuildPlan(config); err != nil {
		t.Fatal(err)
	}
	config.Limits = Limits{MaxAgentBundles: MaxBundles, MaxBundleBytes: 64 << 20, MaxTotalBytes: MaxTotalBytes, JobTimeoutSeconds: 1800, RetentionSeconds: 86400, DownloadTTLSeconds: 3600}
	if _, err := BuildPlan(config); err != nil {
		t.Fatal(err)
	}
}
