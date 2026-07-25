# Security policy

Report suspected vulnerabilities privately to the organization maintainers. Do not attach production diagnostics, access tokens, pseudonymous agent identifiers, or bundles to a public issue.

Every snapshot, summary, bundle creation, list, download, and deletion request requires a constant-time bearer-token comparison. The deployment token must be random, unique, at least 32 characters long, and rotated after suspected disclosure. The service never includes it in logs, files, URLs, HTML, or responses.

The service accepts one strict JSON document of at most 64 KiB. Unknown fields, extra documents, invalid pseudonyms, unreasonable timestamps, unsupported versions, and out-of-range counters are rejected. Agent-side assertions are always revalidated.

Snapshots and bundles are stored only under the configured dedicated directory. Names are cryptographically random or generated internally, paths are not accepted from clients, files use mode `0600`, and directories use mode `0700`. Per-agent history, total agent count, retention, request size, and bundle size are bounded. Operators can delete any generated bundle immediately.

The supplied image has no shell, runs as numeric user `65532`, and requires no privileged mode, host namespace, Linux capability, host mount, or container-engine socket. Place the service only on the managed internal network and use TLS whenever traffic crosses an untrusted network.
