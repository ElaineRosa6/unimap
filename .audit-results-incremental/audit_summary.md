# QA Security Audit Report

**Generated**: 2026-07-06T06:17:12.609487+00:00
**Total Findings**: 64

## Severity Overview

| Severity | Count |
|---|---|
| P0 (Critical) | 0 |
| P1 (High) | 2 |
| P2 (Medium) | 62 |

## Suppressions and Baseline

- Active rules: 0
- Suppressed findings: 0
- Sources: none

## Scanner Results

| Scanner | Status | Duration | Findings |
|---|---|---|---|
| structure_analyzer | success | 949ms | 0 |
| static_analyzer | success | 1183ms | 1596 |
| security_scanner | success | 4833ms | 43 |
| coverage_matrix | success | 200ms | 0 |
| external_security_scanner | success | 175011ms | 112 |
| secret_history_scanner | success | 246ms | 1 |
| manual_review_planner | success | 2225ms | 119 |
| ai_manual_review_runner | success | 100543ms | 0 |
| contract_checker | success | 3385ms | 46 |
| dep_scanner | success | 9669ms | 0 |
| complexity_scanner | success | 1605ms | 232 |
| testability_scanner | success | 100607ms | 22 |
| duplication_scanner | success | 2057ms | 2043 |
| business_flow_scanner | success | 1681ms | 17 |
| error_handling_scanner | success | 9248ms | 142 |
| observability_scanner | success | 777ms | 185 |
| agent_skill_security_scanner | success | 2296ms | 239 |

## ⚠️ Dependency CVE Coverage Gap

`dep_scanner` has **no offline CVE database** for: `go`. Dependencies in these ecosystems were parsed but **NOT** checked for known vulnerabilities — an empty vulnerability list means *not checked*, not *clean*.

**Mandatory follow-up** (treat output as authoritative scanner evidence): `govulncheck ./...` (Go) · `cargo audit` (Rust).

## Scanner Coverage Matrix

| Coverage Status | Count |
|---|---|
| automated | 19 |
| ai-first | 8 |
| external-tool | 8 |

## Manual Review Targets

These are not confirmed vulnerabilities. They mark code that requires AI-agent authorization, state-machine, interleaving, lifetime, or taint-flow verification first. Escalate to human review only when the agent cannot close the evidence chain.

| Class | Severity | Location | AI Review Stage | Verification Focus |
|---|---|---|---|---|
| data_flow_path_traversal | P1 | `urlive.py:457` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| data_flow_path_traversal | P1 | `cmd/unimap-cli/cli_test.go:51` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| race_condition_or_toctou | P1 | `cmd/unimap-gui/main.go:264` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `cmd/unimap-gui/monitor_native.go:314` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P1 | `cmd/unimap-web/main.go:85` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `docs/test_reports/settings_script.js:177` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| data_flow_xss | P1 | `docs/test_reports/settings_script.js:364` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| data_flow_ssrf | P1 | `docs/test_reports/settings_script.js:19` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| race_condition_or_toctou | P1 | `internal/adapter/icp.go:495` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| state_machine_logic | P1 | `internal/adapter/orchestrator_circuit_test.go:22` | AI_AGENT_REVIEW_PENDING | List all allowed states and transitions. |
| race_condition_or_toctou | P1 | `internal/adapter/orchestrator_circuit_test.go:115` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `internal/adapter/orchestrator_search.go:205` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P1 | `internal/adapter/orchestrator_test.go:249` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `internal/alerting/manager.go:129` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| data_flow_path_traversal | P1 | `internal/auth/api_key_test.go:20` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| race_condition_or_toctou | P2 | `internal/auth/permission_test.go:335` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| data_flow_path_traversal | P1 | `internal/config/config_coverage_test.go:350` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| data_flow_path_traversal | P1 | `internal/config/config_test.go:214` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| state_machine_logic | P1 | `internal/distributed/task_queue_test.go:28` | AI_AGENT_REVIEW_PENDING | List all allowed states and transitions. |
| data_flow_sql_injection | P1 | `internal/history/repository.go:69` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| data_flow_command_injection | P1 | `internal/history/repository.go:48` | AI_AGENT_REVIEW_PENDING | Trace each source value through assignments, function calls, serializers, and validators. |
| race_condition_or_toctou | P2 | `internal/logger/alert_hook_test.go:118` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `internal/monitoring/leak_detector.go:55` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `internal/monitoring/monitoring_test.go:189` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| race_condition_or_toctou | P2 | `internal/monitoring/resource_monitor.go:145` | AI_AGENT_REVIEW_PENDING | List every shared variable read/written by concurrent tasks. |
| authorization_scope | P1 | `internal/notify/channels_test.go:349` | AI_AGENT_REVIEW_PENDING | Is the resource loaded through the authenticated principal rather than a client-supplied id? |
| authorization_scope | P1 | `internal/notify/channels_test.go:397` | AI_AGENT_REVIEW_PENDING | Is the resource loaded through the authenticated principal rather than a client-supplied id? |
| authorization_scope | P1 | `internal/notify/channels_test.go:400` | AI_AGENT_REVIEW_PENDING | Is the resource loaded through the authenticated principal rather than a client-supplied id? |
| authorization_scope | P1 | `internal/notify/channels_test.go:681` | AI_AGENT_REVIEW_PENDING | Is the resource loaded through the authenticated principal rather than a client-supplied id? |
| state_machine_logic | P1 | `internal/notify/channels_test.go:96` | AI_AGENT_REVIEW_PENDING | List all allowed states and transitions. |

## AI-First Review Packets

The harness generated AI-agent review packets with nearby code context and suggested-grep evidence. These packets are not final verdicts; the AI agent must fill the result slots before human review escalation.

- Packets ready: 119
- Pending AI review: 119

| Target | Class | Severity | Location |
|---|---|---|---|
| `AI-REVIEW-001-data_flow_path_traversal-urlive.py-457` | data_flow_path_traversal | P1 | `urlive.py:457` |
| `AI-REVIEW-002-data_flow_path_traversal-cmd_unimap-cli_cli_test.go-51` | data_flow_path_traversal | P1 | `cmd/unimap-cli/cli_test.go:51` |
| `AI-REVIEW-003-race_condition_or_toctou-cmd_unimap-gui_main.go-264` | race_condition_or_toctou | P1 | `cmd/unimap-gui/main.go:264` |
| `AI-REVIEW-004-race_condition_or_toctou-cmd_unimap-gui_monitor_native.go-314` | race_condition_or_toctou | P2 | `cmd/unimap-gui/monitor_native.go:314` |
| `AI-REVIEW-005-race_condition_or_toctou-cmd_unimap-web_main.go-85` | race_condition_or_toctou | P1 | `cmd/unimap-web/main.go:85` |
| `AI-REVIEW-006-race_condition_or_toctou-docs_test_reports_settings_script.js-177` | race_condition_or_toctou | P2 | `docs/test_reports/settings_script.js:177` |
| `AI-REVIEW-007-data_flow_xss-docs_test_reports_settings_script.js-364` | data_flow_xss | P1 | `docs/test_reports/settings_script.js:364` |
| `AI-REVIEW-008-data_flow_ssrf-docs_test_reports_settings_script.js-19` | data_flow_ssrf | P1 | `docs/test_reports/settings_script.js:19` |
| `AI-REVIEW-009-race_condition_or_toctou-internal_adapter_icp.go-495` | race_condition_or_toctou | P1 | `internal/adapter/icp.go:495` |
| `AI-REVIEW-010-state_machine_logic-internal_adapter_orchestrator_circuit_test.go-22` | state_machine_logic | P1 | `internal/adapter/orchestrator_circuit_test.go:22` |
| `AI-REVIEW-011-race_condition_or_toctou-internal_adapter_orchestrator_circuit_test.go-115` | race_condition_or_toctou | P1 | `internal/adapter/orchestrator_circuit_test.go:115` |
| `AI-REVIEW-012-race_condition_or_toctou-internal_adapter_orchestrator_search.go-205` | race_condition_or_toctou | P2 | `internal/adapter/orchestrator_search.go:205` |
| `AI-REVIEW-013-race_condition_or_toctou-internal_adapter_orchestrator_test.go-249` | race_condition_or_toctou | P1 | `internal/adapter/orchestrator_test.go:249` |
| `AI-REVIEW-014-race_condition_or_toctou-internal_alerting_manager.go-129` | race_condition_or_toctou | P2 | `internal/alerting/manager.go:129` |
| `AI-REVIEW-015-data_flow_path_traversal-internal_auth_api_key_test.go-20` | data_flow_path_traversal | P1 | `internal/auth/api_key_test.go:20` |
| `AI-REVIEW-016-race_condition_or_toctou-internal_auth_permission_test.go-335` | race_condition_or_toctou | P2 | `internal/auth/permission_test.go:335` |
| `AI-REVIEW-017-data_flow_path_traversal-internal_config_config_coverage_test.go-350` | data_flow_path_traversal | P1 | `internal/config/config_coverage_test.go:350` |
| `AI-REVIEW-018-data_flow_path_traversal-internal_config_config_test.go-214` | data_flow_path_traversal | P1 | `internal/config/config_test.go:214` |
| `AI-REVIEW-019-state_machine_logic-internal_distributed_task_queue_test.go-28` | state_machine_logic | P1 | `internal/distributed/task_queue_test.go:28` |
| `AI-REVIEW-020-data_flow_sql_injection-internal_history_repository.go-69` | data_flow_sql_injection | P1 | `internal/history/repository.go:69` |
| `AI-REVIEW-021-data_flow_command_injection-internal_history_repository.go-48` | data_flow_command_injection | P1 | `internal/history/repository.go:48` |
| `AI-REVIEW-022-race_condition_or_toctou-internal_logger_alert_hook_test.go-118` | race_condition_or_toctou | P2 | `internal/logger/alert_hook_test.go:118` |
| `AI-REVIEW-023-race_condition_or_toctou-internal_monitoring_leak_detector.go-55` | race_condition_or_toctou | P2 | `internal/monitoring/leak_detector.go:55` |
| `AI-REVIEW-024-race_condition_or_toctou-internal_monitoring_monitoring_test.go-189` | race_condition_or_toctou | P2 | `internal/monitoring/monitoring_test.go:189` |
| `AI-REVIEW-025-race_condition_or_toctou-internal_monitoring_resource_monitor.go-145` | race_condition_or_toctou | P2 | `internal/monitoring/resource_monitor.go:145` |
| `AI-REVIEW-026-authorization_scope-internal_notify_channels_test.go-349` | authorization_scope | P1 | `internal/notify/channels_test.go:349` |
| `AI-REVIEW-027-authorization_scope-internal_notify_channels_test.go-397` | authorization_scope | P1 | `internal/notify/channels_test.go:397` |
| `AI-REVIEW-028-authorization_scope-internal_notify_channels_test.go-400` | authorization_scope | P1 | `internal/notify/channels_test.go:400` |
| `AI-REVIEW-029-authorization_scope-internal_notify_channels_test.go-681` | authorization_scope | P1 | `internal/notify/channels_test.go:681` |
| `AI-REVIEW-030-state_machine_logic-internal_notify_channels_test.go-96` | state_machine_logic | P1 | `internal/notify/channels_test.go:96` |

## Top Findings (sorted by severity)

### [P1] No test directories found
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add test directories and implement unit/integration tests

### [P1] No health-check endpoint found (/health, /healthz, /readyz, or Spring actuator) — orchestrators cannot determine service liveness
- **Location**: N/A
- **Scanner**: observability_scanner
- **Suggestion**: Add a /health or /healthz endpoint that checks critical dependencies (DB, cache) and returns 200/503 accordingly

### [P2] Source directory 'cmd\unimap-cli' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'cmd\unimap-cli'

### [P2] Source directory 'cmd\unimap-gui' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'cmd\unimap-gui'

### [P2] Source directory 'cmd\unimap-web' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'cmd\unimap-web'

### [P2] Source directory 'docs\test_reports' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'docs\test_reports'

### [P2] Source directory 'internal\adapter' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\adapter'

### [P2] Source directory 'internal\alerting' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\alerting'

### [P2] Source directory 'internal\appversion' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\appversion'

### [P2] Source directory 'internal\auth' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\auth'

### [P2] Source directory 'internal\backup' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\backup'

### [P2] Source directory 'internal\collection' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\collection'

### [P2] Source directory 'internal\config' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\config'

### [P2] Source directory 'internal\core\unimap' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\core\unimap'

### [P2] Source directory 'internal\distributed' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\distributed'

### [P2] Source directory 'internal\error' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\error'

### [P2] Source directory 'internal\exporter' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\exporter'

### [P2] Source directory 'internal\history' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\history'

### [P2] Source directory 'internal\icp\database' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\icp\database'

### [P2] Source directory 'internal\logger' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\logger'

### [P2] Source directory 'internal\metrics' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\metrics'

### [P2] Source directory 'internal\model' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\model'

### [P2] Source directory 'internal\monitoring' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\monitoring'

### [P2] Source directory 'internal\notify' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\notify'

### [P2] Source directory 'internal\plugin' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\plugin'

### [P2] Source directory 'internal\plugin\processors' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\plugin\processors'

### [P2] Source directory 'internal\proxypool' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\proxypool'

### [P2] Source directory 'internal\requestid' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\requestid'

### [P2] Source directory 'internal\scheduler' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\scheduler'

### [P2] Source directory 'internal\screenshot' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\screenshot'

### [P2] Source directory 'internal\screenshot\batchdb' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\screenshot\batchdb'

### [P2] Source directory 'internal\service' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\service'

### [P2] Source directory 'internal\tamper' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper'

### [P2] Source directory 'internal\tamper\analyzer' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\analyzer'

### [P2] Source directory 'internal\tamper\database' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\database'

### [P2] Source directory 'internal\tamper\decoder' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\decoder'

### [P2] Source directory 'internal\tamper\fingerprint' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\fingerprint'

### [P2] Source directory 'internal\tamper\performance' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\performance'

### [P2] Source directory 'internal\tamper\priority' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\priority'

### [P2] Source directory 'internal\tamper\threshold' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\tamper\threshold'

### [P2] Source directory 'internal\utils' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\utils'

### [P2] Source directory 'internal\utils\circuitbreaker' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\utils\circuitbreaker'

### [P2] Source directory 'internal\utils\degradation' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\utils\degradation'

### [P2] Source directory 'internal\utils\urlguard' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\utils\urlguard'

### [P2] Source directory 'internal\utils\workerpool' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'internal\utils\workerpool'

### [P2] Source directory 'tools\extension-screenshot' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'tools\extension-screenshot'

### [P2] Source directory 'tools\extension-screenshot\src' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'tools\extension-screenshot\src'

### [P2] Source directory 'web\static\js' has no sibling or nested test directory
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Add tests/ or __tests__/ alongside 'web\static\js'

### [P2] Module 'bytes' is imported by 26 files (threshold 20) — god-module / change-magnet
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Split the module into cohesive submodules. A module imported everywhere has accumulated unrelated responsibilities and changes ripple across the codebase

### [P2] Module 'url' is imported by 26 files (threshold 20) — god-module / change-magnet
- **Location**: N/A
- **Scanner**: structure_analyzer
- **Suggestion**: Split the module into cohesive submodules. A module imported everywhere has accumulated unrelated responsibilities and changes ripple across the codebase
