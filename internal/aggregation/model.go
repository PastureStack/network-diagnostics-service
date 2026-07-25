package aggregation

const (
	InputSchema = "pasturestack.diagnostics-aggregation/v1"
	PlanSchema  = "pasturestack.diagnostics-aggregation-plan/v1"

	MaxInputBytes = 2 << 20
	MaxBundles    = 256
	MaxTotalBytes = 512 << 20
)

type Config struct {
	Schema               string        `json:"schema"`
	CollectionPlanSHA256 string        `json:"collection_plan_sha256"`
	Authorization        Authorization `json:"authorization"`
	Limits               Limits        `json:"limits"`
	Bundles              []Bundle      `json:"bundles"`
}

type Authorization struct {
	OperatorAuthenticated bool `json:"operator_authenticated"`
	SensitiveDataApproved bool `json:"sensitive_data_approved"`
}

type Limits struct {
	MaxAgentBundles    int   `json:"max_agent_bundles"`
	MaxBundleBytes     int64 `json:"max_bundle_bytes"`
	MaxTotalBytes      int64 `json:"max_total_bytes"`
	JobTimeoutSeconds  int   `json:"job_timeout_seconds"`
	RetentionSeconds   int   `json:"retention_seconds"`
	DownloadTTLSeconds int   `json:"download_ttl_seconds"`
}

type Bundle struct {
	ManifestSHA256  string `json:"manifest_sha256"`
	ContentSHA256   string `json:"content_sha256"`
	Bytes           int64  `json:"bytes"`
	ProbeCount      int    `json:"probe_count"`
	HighSensitivity bool   `json:"high_sensitivity"`
	RedactionStatus string `json:"redaction_status"`
	Encrypted       bool   `json:"encrypted"`
}

type Plan struct {
	Schema           string        `json:"schema"`
	NormalizedSHA256 string        `json:"normalized_sha256"`
	Authorization    Authorization `json:"authorization_assertions"`
	Limits           Limits        `json:"limits"`
	Bundles          BundleSummary `json:"bundles"`
	Safeguards       Safeguards    `json:"safeguards"`
}

type BundleSummary struct {
	Total           int   `json:"total"`
	HighSensitivity int   `json:"high_sensitivity"`
	TotalBytes      int64 `json:"total_bytes"`
}

type Safeguards struct {
	Mode                            string `json:"mode"`
	OpensNetworkListener            bool   `json:"opens_network_listener"`
	ContactsAgents                  bool   `json:"contacts_agents"`
	ExecutesCommands                bool   `json:"executes_commands"`
	ReadsOrWritesBundles            bool   `json:"reads_or_writes_bundles"`
	IssuesDownloadGrants            bool   `json:"issues_download_grants"`
	AcceptsTargetIdentifiers        bool   `json:"accepts_target_identifiers"`
	AuthenticatedOperatorVerified   bool   `json:"authenticated_operator_verified"`
	SensitiveDataApprovalVerified   bool   `json:"sensitive_data_approval_verified"`
	ManifestSignaturesVerified      bool   `json:"manifest_signatures_verified"`
	AuthenticatedEncryptionVerified bool   `json:"authenticated_encryption_verified"`
	DownloadAuditImplemented        bool   `json:"download_audit_implemented"`
	SingleUseDownloadImplemented    bool   `json:"single_use_download_implemented"`
	AllowsDirectoryListing          bool   `json:"allows_directory_listing"`
	ProductionReady                 bool   `json:"production_ready"`
}
