# Operations

Running and maintaining the AI Document Processing Agent after
[Deployment](DEPLOYMENT.md).

For local development (starting/stopping the Docker Compose stack,
rebuilding after code changes, scaling the worker), see the
[README's "Operating with Docker Compose" section](../README.md#operating-with-docker-compose) —
this document covers production operations.

## Monitoring & Alerting

| Signal | Tool | Threshold | Alert Channel |
| --- | --- | --- | --- |
| API availability | Uptime check | < 99% monthly (NFR-14) | TBD |
| Upload → ack latency | Metrics/APM | > 2s p95 (NFR-1) | TBD |
| Processing duration | Metrics/APM | > 60s p95 (NFR-2) | TBD |
| Agent run failure rate | Metrics/APM | > 5% (PRD success metric) | TBD |
| Review queue backlog | Metrics/APM | Growing unbounded | TBD |
| Tool timeout rate | Metrics/APM | Elevated vs. baseline | TBD |

## On-Call

- TBD: rotation and escalation policy.

## Runbooks

| Scenario | Runbook |
| --- | --- |
| Agent runs stuck at max iterations for many documents | Check LLM/OCR provider status; verify tool timeouts are not too aggressive |
| Review queue backlog growing | Check reviewer capacity; check for a spike in low-confidence classifications (possible provider regression) |
| Elevated `FAILED` document rate | Check provider error rates and queue worker health; inspect recent `tool_executions` for a common failure mode |
| Duplicate detection false positives | Review `check_duplicate` thresholds/rules |

## Incident Response

- Every agent run has a `trace_id` (NFR-19); use it to correlate API logs, worker logs, and `tool_executions` rows when investigating an incident.
- Audit log (`audit_logs`) is the source of truth for what the agent and reviewers did, in what order — check it first when reconstructing an incident timeline.

## Maintenance

- Backups: TBD (Postgres backup/restore policy).
- Dependency updates: TBD (Go module and Next.js dependency update cadence).
