package aggregation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type normalizedConfig struct {
	Schema               string        `json:"schema"`
	CollectionPlanSHA256 string        `json:"collection_plan_sha256"`
	Authorization        Authorization `json:"authorization"`
	Limits               Limits        `json:"limits"`
	Bundles              []Bundle      `json:"bundles"`
}

func normalize(config Config) (normalizedConfig, error) {
	if config.Schema != InputSchema {
		return normalizedConfig{}, fmt.Errorf("schema must be %q", InputSchema)
	}
	collectionDigest, err := normalizeDigest(config.CollectionPlanSHA256)
	if err != nil {
		return normalizedConfig{}, errors.New("collection_plan_sha256 must be a 64-character hexadecimal digest")
	}
	if !config.Authorization.OperatorAuthenticated {
		return normalizedConfig{}, errors.New("authorization.operator_authenticated must be true")
	}
	if config.Limits.MaxAgentBundles < 1 || config.Limits.MaxAgentBundles > MaxBundles {
		return normalizedConfig{}, fmt.Errorf("limits.max_agent_bundles must be between 1 and %d", MaxBundles)
	}
	if config.Limits.MaxBundleBytes < 64<<10 || config.Limits.MaxBundleBytes > 64<<20 {
		return normalizedConfig{}, errors.New("limits.max_bundle_bytes must be between 65536 and 67108864")
	}
	if config.Limits.MaxTotalBytes < config.Limits.MaxBundleBytes || config.Limits.MaxTotalBytes > MaxTotalBytes {
		return normalizedConfig{}, fmt.Errorf("limits.max_total_bytes must be at least max_bundle_bytes and no more than %d", MaxTotalBytes)
	}
	if config.Limits.JobTimeoutSeconds < 60 || config.Limits.JobTimeoutSeconds > 1800 {
		return normalizedConfig{}, errors.New("limits.job_timeout_seconds must be between 60 and 1800")
	}
	if config.Limits.RetentionSeconds < 300 || config.Limits.RetentionSeconds > 86400 {
		return normalizedConfig{}, errors.New("limits.retention_seconds must be between 300 and 86400")
	}
	if config.Limits.DownloadTTLSeconds < 60 || config.Limits.DownloadTTLSeconds > 3600 || config.Limits.DownloadTTLSeconds > config.Limits.RetentionSeconds {
		return normalizedConfig{}, errors.New("limits.download_ttl_seconds is outside the allowed range")
	}
	if len(config.Bundles) < 1 || len(config.Bundles) > config.Limits.MaxAgentBundles || len(config.Bundles) > MaxBundles {
		return normalizedConfig{}, errors.New("bundles count is outside the allowed range")
	}

	bundles := append([]Bundle(nil), config.Bundles...)
	manifestDigests := make(map[string]struct{}, len(bundles))
	contentDigests := make(map[string]struct{}, len(bundles))
	var totalBytes int64
	for index := range bundles {
		bundle := &bundles[index]
		bundle.ManifestSHA256, err = normalizeDigest(bundle.ManifestSHA256)
		if err != nil {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].manifest_sha256 is invalid", index)
		}
		bundle.ContentSHA256, err = normalizeDigest(bundle.ContentSHA256)
		if err != nil {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].content_sha256 is invalid", index)
		}
		if _, exists := manifestDigests[bundle.ManifestSHA256]; exists {
			return normalizedConfig{}, fmt.Errorf("bundles[%d] duplicates a manifest digest", index)
		}
		manifestDigests[bundle.ManifestSHA256] = struct{}{}
		if _, exists := contentDigests[bundle.ContentSHA256]; exists {
			return normalizedConfig{}, fmt.Errorf("bundles[%d] duplicates a content digest", index)
		}
		contentDigests[bundle.ContentSHA256] = struct{}{}
		if bundle.Bytes < 1 || bundle.Bytes > config.Limits.MaxBundleBytes {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].bytes is outside the allowed range", index)
		}
		if bundle.ProbeCount < 1 || bundle.ProbeCount > 32 {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].probe_count must be between 1 and 32", index)
		}
		if bundle.RedactionStatus != "passed" {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].redaction_status must be passed", index)
		}
		if !bundle.Encrypted {
			return normalizedConfig{}, fmt.Errorf("bundles[%d].encrypted must be true", index)
		}
		if bundle.HighSensitivity && !config.Authorization.SensitiveDataApproved {
			return normalizedConfig{}, fmt.Errorf("bundles[%d] requires sensitive-data approval", index)
		}
		totalBytes += bundle.Bytes
		if totalBytes > config.Limits.MaxTotalBytes {
			return normalizedConfig{}, errors.New("bundle bytes exceed limits.max_total_bytes")
		}
	}
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].ManifestSHA256 < bundles[j].ManifestSHA256 })
	return normalizedConfig{Schema: config.Schema, CollectionPlanSHA256: collectionDigest, Authorization: config.Authorization, Limits: config.Limits, Bundles: bundles}, nil
}

func normalizeDigest(value string) (string, error) {
	if len(value) != 64 {
		return "", errors.New("invalid digest length")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", err
	}
	return strings.ToLower(value), nil
}
