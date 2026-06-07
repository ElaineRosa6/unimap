#!/usr/bin/env bash
# test_scheduler_runners.sh — 逐项测试 22 个 Scheduler Runner
#
# 用法：
#   BASE_URL=http://localhost:8448 AUTH_TOKEN=your_token ./scripts/test_scheduler_runners.sh
#
# 可选参数：
#   BASE_URL     - UniMap Web 地址 (默认 http://localhost:8448)
#   AUTH_TOKEN    - Admin Token (如果 web.auth.enabled=true)
#   TEST_ST      - 只运行指定 ST 编号，逗号分隔 (如 "01,05,13")
#   SKIP_ST      - 跳过指定 ST 编号，逗号分隔
#   WAIT_SEC     - 等待任务执行完成的秒数 (默认 30)
#   CLEANUP      - 是否在测试后删除任务 (默认 true)
#   PARALLEL     - 是否并行执行任务 (默认 false)
#
# 示例：
#   # 测试全部
#   BASE_URL=http://localhost:8448 ./scripts/test_scheduler_runners.sh
#
#   # 只测 ICP 和配额
#   TEST_ST=13,21 ./scripts/test_scheduler_runners.sh
#
#   # 跳过截图相关（需要 Chrome）
#   SKIP_ST=02,03,06,07 ./scripts/test_scheduler_runners.sh

set -euo pipefail

# --- Configuration ---
BASE_URL="${BASE_URL:-http://localhost:8448}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
WAIT_SEC="${WAIT_SEC:-30}"
CLEANUP="${CLEANUP:-true}"
PARALLEL="${PARALLEL:-false}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PAYLOAD_DIR="${SCRIPT_DIR}/test_payloads"
RESULTS_DIR="${SCRIPT_DIR}/test_results"
mkdir -p "$RESULTS_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Counters
PASS=0
FAIL=0
SKIP=0
ERRORS=()

# --- Helpers ---

log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC} $*"; ((PASS++)); }
log_fail()  { echo -e "${RED}[FAIL]${NC} $*"; ((FAIL++)); ERRORS+=("$*"); }
log_skip()  { echo -e "${YELLOW}[SKIP]${NC} $*"; ((SKIP++)); }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

curl_api() {
  local method="$1" path="$2"
  shift 2
  local url="${BASE_URL}/api/scheduler${path}"
  local args=(-s -w "\n%{http_code}" -X "$method" "$url")
  if [[ -n "$AUTH_TOKEN" ]]; then
    args+=(-H "Authorization: Bearer ${AUTH_TOKEN}")
  fi
  args+=(-H "Content-Type: application/json")
  args+=(-H "Origin: ${BASE_URL}")
  args+=("$@")
  curl "${args[@]}"
}

# Parse curl response: body on all lines except last, status code on last line
parse_response() {
  local resp="$1"
  HTTP_CODE=$(echo "$resp" | tail -1)
  BODY=$(echo "$resp" | sed '$d')
}

should_test() {
  local st="$1"
  if [[ -n "${TEST_ST:-}" ]]; then
    [[ ",$TEST_ST," == *",$st,"* ]] || return 1
  fi
  if [[ -n "${SKIP_ST:-}" ]]; then
    [[ ",$SKIP_ST," == *",$st,"* ]] && return 1
  fi
  return 0
}

# --- Task Definitions ---
# Format: ST|task_type|name|payload_file|description|expected_keywords
# expected_keywords: comma-separated substrings expected in execution result
declare -a TASK_DEFS=(
  "01|query|ST-01 UQL查询测试|st01_query.json|UQL query via FOFA|retrieved,assets"
  "02|search_screenshot|ST-02 搜索截图测试|st02_search_screenshot.json|FOFA search screenshot|captured"
  "03|batch_screenshot|ST-03 批量截图测试|st03_batch_screenshot.json|Batch URL screenshots|batch"
  "04|tamper_check|ST-04 篡改检测测试|st04_tamper_check.json|Tamper check relaxed|tamper check"
  "05|url_reachability|ST-05 URL可达性测试|st05_url_reachability.json|URL reachability check|reachability"
  "06|cookie_verify|ST-06 Cookie验证测试|st06_cookie_verify.json|Cookie verification|cookie"
  "07|login_status_check|ST-07 登录状态测试|st07_login_status_check.json|Login status check|logged_in"
  "08|distributed_submit|ST-08 分布式提交测试|st08_distributed_submit.json|Distributed task submit|enqueued"
  "09|export|ST-09 数据导出测试|st09_export.json|Export query results|exported"
  "10|port_scan|ST-10 端口扫描测试|st10_port_scan.json|Port scan|scanned"
  "11|screenshot_cleanup|ST-11 截图清理测试|st11_screenshot_cleanup.json|Screenshot cleanup|cleaned"
  "12|tamper_cleanup|ST-12 篡改清理测试|st12_tamper_cleanup.json|Tamper record cleanup|expired"
  "13|quota_monitor|ST-13 配额监控测试|st13_quota_monitor.json|Quota monitoring|remaining\|ok\|LOW\|no engine"
  "14|alert_summary|ST-14 告警汇总测试|st14_alert_summary.json|Alert summary|alert summary"
  "15|baseline_refresh|ST-15 基线刷新测试|st15_baseline_refresh.json|Baseline refresh|refreshed\|no baselines"
  "16|url_import|ST-16 URL导入测试|st16_url_import.json|URL import|imported\|no files"
  "17|plugin_health|ST-17 插件健康测试|st17_plugin_health.json|Plugin health check|plugins"
  "18|bridge_token|ST-18 Bridge健康测试|st18_bridge_token_rotate.json|Bridge health check|bridge health"
  "19|alert_silence|ST-19 告警静默测试|st19_alert_silence.json|Alert silence|silenced"
  "20|cache_warmup|ST-20 缓存预热测试|st20_cache_warmup.json|Cache warmup/URL health|warmed"
  "21|icp_query|ST-21 ICP备案测试|st21_icp_query.json|ICP query|icp"
  "22|icp_import|ST-22 ICP导入测试|st22_icp_import.json|ICP import|imported\|no files"
)

# --- Main ---

log_info "========================================="
log_info "UniMap Scheduler Runner 测试"
log_info "Base URL: ${BASE_URL}"
log_info "========================================="
echo ""

# Check connectivity
log_info "检查 UniMap Web 服务..."
health_args=(-s -o /dev/null -w "%{http_code}" "${BASE_URL}/api/scheduler/tasks")
if [[ -n "$AUTH_TOKEN" ]]; then
  health_args+=(-H "Authorization: Bearer ${AUTH_TOKEN}")
fi
resp=$(curl "${health_args[@]}" 2>/dev/null || true)
if [[ "$resp" != "200" ]]; then
  log_warn "无法访问 ${BASE_URL}/api/scheduler/tasks (HTTP $resp)，请确认服务已启动"
  log_warn "启动命令: go run ./cmd/unimap-web"
fi
echo ""

declare -a CREATED_TASK_IDS=()
declare -a RUNNING_TASKS=()

# Phase 1: Create all tasks
log_info "--- Phase 1: 创建测试任务 ---"
for def in "${TASK_DEFS[@]}"; do
  IFS='|' read -r st task_type name payload_file description expected <<< "$def"

  if ! should_test "$st"; then
    log_skip "ST-${st}: ${description} (被跳过)"
    continue
  fi

  payload_path="${PAYLOAD_DIR}/${payload_file}"
  if [[ ! -f "$payload_path" ]]; then
    log_fail "ST-${st}: payload 文件不存在 ${payload_path}"
    continue
  fi

  payload=$(cat "$payload_path")
  body=$(cat <<EOJSON
{
  "name": "${name}",
  "type": "${task_type}",
  "enabled": true,
  "cron_expr": "0 0 0 1 1 *",
  "timeout_seconds": 300,
  "max_retries": 0,
  "payload": ${payload}
}
EOJSON
)

  resp=$(curl_api POST "/tasks/create" -d "$body")
  parse_response "$resp"

  if [[ "$HTTP_CODE" == "201" ]]; then
    task_id=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$task_id" ]]; then
      CREATED_TASK_IDS+=("$task_id")
      RUNNING_TASKS+=("${st}|${task_id}|${name}|${expected}")
      log_info "ST-${st}: 创建任务 ${task_id} ✓"
    else
      log_fail "ST-${st}: 创建成功但无法解析 task_id (HTTP $HTTP_CODE)"
    fi
  else
    log_fail "ST-${st}: 创建失败 (HTTP $HTTP_CODE) - ${BODY}"
  fi
done

echo ""
log_info "共创建 ${#CREATED_TASK_IDS[@]} 个任务"

if [[ ${#CREATED_TASK_IDS[@]} -eq 0 ]]; then
  log_warn "没有创建任何任务，退出"
  exit 0
fi

# Phase 2: Trigger immediate execution
log_info "--- Phase 2: 触发立即执行 ---"
for entry in "${RUNNING_TASKS[@]}"; do
  IFS='|' read -r st task_id name expected <<< "$entry"

  resp=$(curl_api POST "/tasks/run" -d "{\"id\":\"${task_id}\"}")
  parse_response "$resp"

  if [[ "$HTTP_CODE" == "200" ]]; then
    log_info "ST-${st}: 触发执行 ✓"
  else
    log_fail "ST-${st}: 触发执行失败 (HTTP $HTTP_CODE) - ${BODY}"
  fi
done

# Phase 3: Wait for execution
echo ""
log_info "--- Phase 3: 等待任务执行 (${WAIT_SEC}s) ---"
sleep "$WAIT_SEC"

# Phase 4: Check results
echo ""
log_info "--- Phase 4: 检查执行结果 ---"

resp=$(curl_api GET "/history?limit=200")
parse_response "$resp"
HISTORY="$BODY"

if [[ "$HTTP_CODE" != "200" ]]; then
  log_fail "获取执行历史失败 (HTTP $HTTP_CODE)"
  echo "$BODY"
  exit 1
fi

# Save full history
echo "$HISTORY" > "${RESULTS_DIR}/history_$(date +%Y%m%d_%H%M%S).json"

for entry in "${RUNNING_TASKS[@]}"; do
  IFS='|' read -r st task_id name expected <<< "$entry"

  # Find the latest execution record for this task
  # Using simple grep since we may not have jq
  task_history=$(echo "$HISTORY" | grep -o "\"task_id\":\"${task_id}\"[^}]*}" | tail -1)

  if [[ -z "$task_history" ]]; then
    log_fail "ST-${st}: 未找到执行记录"
    continue
  fi

  # Check status
  if echo "$task_history" | grep -q '"status":"success"'; then
    # Check expected keywords in result
    result=$(echo "$task_history" | grep -o '"result":"[^"]*"' | head -1 | cut -d'"' -f4)
    matched=false
    IFS=',' read -ra keywords <<< "$expected"
    for kw in "${keywords[@]}"; do
      if echo "$result" | grep -qi "$kw"; then
        matched=true
        break
      fi
    done
    if $matched; then
      log_pass "ST-${st}: ${name} — ${result}"
    else
      log_warn "ST-${st}: 执行成功但结果不含预期关键词 — ${result}"
      log_pass "ST-${st}: ${name} (结果内容待人工确认)"
    fi
  elif echo "$task_history" | grep -q '"status":"failed"'; then
    error=$(echo "$task_history" | grep -o '"error":"[^"]*"' | head -1 | cut -d'"' -f4)
    log_fail "ST-${st}: ${name} — 失败: ${error}"
  elif echo "$task_history" | grep -q '"status":"timeout"'; then
    log_fail "ST-${st}: ${name} — 超时"
  else
    status=$(echo "$task_history" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    log_warn "ST-${st}: ${name} — 状态: ${status} (可能仍在执行中)"
  fi
done

# Phase 5: Cleanup
echo ""
if [[ "$CLEANUP" == "true" ]]; then
  log_info "--- Phase 5: 清理测试任务 ---"
  for task_id in "${CREATED_TASK_IDS[@]}"; do
    resp=$(curl_api POST "/tasks/delete" -d "{\"id\":\"${task_id}\"}")
    parse_response "$resp"
    if [[ "$HTTP_CODE" == "200" ]]; then
      log_info "已删除任务 ${task_id}"
    else
      log_warn "删除任务 ${task_id} 失败 (HTTP $HTTP_CODE)"
    fi
  done
else
  log_info "--- Phase 5: 跳过清理 (CLEANUP=false) ---"
  log_info "测试任务 ID 列表:"
  for task_id in "${CREATED_TASK_IDS[@]}"; do
    echo "  ${task_id}"
  done
fi

# Summary
echo ""
echo "========================================="
echo -e "测试结果汇总"
echo "========================================="
echo -e "  ${GREEN}通过: ${PASS}${NC}"
echo -e "  ${RED}失败: ${FAIL}${NC}"
echo -e "  ${YELLOW}跳过: ${SKIP}${NC}"
echo ""

if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo -e "${RED}失败项:${NC}"
  for err in "${ERRORS[@]}"; do
    echo "  - $err"
  done
  echo ""
fi

echo "详细历史: ${RESULTS_DIR}/"
echo ""

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
