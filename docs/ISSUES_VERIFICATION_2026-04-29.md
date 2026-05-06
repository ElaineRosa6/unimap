# DEEP_CODE_REVIEW_2026-04-29: 25 Issues Verification Report

> Verification date: 2026-04-29
> Scope: All 25 issues checked against actual source code (including uncommitted changes)

---

## CRITICAL (5)

### C-01: auth.enabled default false + bind 0.0.0.0 = unprotected
**Status: NOT FIXED**
configs/config.yaml:78-80: bind_address is 0.0.0.0, auth.enabled is false, admin_token is empty.
Also distributed.admin_token is empty (line 114). No changes made.

### C-02: text/template should be html/template
**Status: NOT FIXED**
web/server.go:16 still imports "text/template". No change to "html/template".

### C-03: RoundRobinScheduler.lastIndex data race
**Status: NOT FIXED**
internal/distributed/scheduler.go:126: lastIndex is still bare int (not int64 + atomic).
Line 147: s.lastIndex++ has no sync/atomic protection.

### C-04: SetRateLimitConfig + sync.Once race
**Status: NOT FIXED**
web/middleware_ratelimit.go:134-138: globalLimiter is bare *RateLimiter, rateLimitEnabled is bare bool.
Lines 244-262: SetRateLimitConfig and SetRateLimitEnabled write without atomic operations.

### C-05: Cookie values leaked into HTML template
**Status: PARTIALLY FIXED**
web/query_handlers.go:142-145: cookieFofa/cookieHunter/cookieQuake/cookieZoomeye (raw cookie strings) are STILL passed to template.
web/query_handlers.go:146-149: cookieHasFofa/cookieHasHunter/cookieHasQuake/cookieHasZoomeye (booleans) were ADDED.
web/templates/index.html:84,87,90,93: Raw cookie values used as value="{{.cookieFofa}}".
The fix is incomplete: boolean flags added but raw values not removed.

---

## HIGH (6)

### H-01: CSP contains unsafe-eval
**Status: NOT FIXED**
web/server.go:497: CSP header still contains 'unsafe-eval'.

### H-02: WebSocket dev mode no auth bypass
**Status: NOT FIXED**
web/websocket_handlers.go:148-152: When UNIMAP_WS_TOKEN is not set, validateWebSocketRequest returns true.

### H-03: SSRF not covering screenshot target URL
**Status: FIXED**
web/screenshot_handlers.go:291-303: handleTargetScreenshot validates req.URL and req.IP via isPrivateOrInternalIP().
Also lines 394-406 (handleBatchScreenshot) and 465-478 (handleBatchURLsScreenshot) have SSRF checks.

### H-04: Alert goroutine no WaitGroup tracking
**Status: NOT FIXED**
internal/alerting/manager.go:101-114: SendAlert launches goroutines without sync.WaitGroup.
Close() at lines 138-147 does not wait.

### H-05: Rate limit default disabled
**Status: NOT FIXED**
configs/config.yaml:103: rate_limit.enabled is false.

### H-06: isTrustedRequest allows empty Origin/Referer for state changes
**Status: FIXED**
web/http_helpers.go:144-151: isStateChange check rejects POST/PUT/PATCH/DELETE when both Origin and Referer are empty.

---

## MEDIUM (9)

### M-01: RateLimiter memory leak - unbounded timestamps
**Status: PARTIALLY FIXED**
web/middleware_ratelimit.go:56-64: Allow() now prunes expired timestamps per-request (correct).
But cleanup() at lines 110-131 still uses window*2 cutoff and only deletes fully-expired entries.

### M-02: File upload only checks extension
**Status: NOT FIXED**
web/monitor_handlers.go:55-67: Only checks file extension, no MIME/magic bytes validation.

### M-03: header.Filename not sanitized
**Status: NOT FIXED**
web/monitor_handlers.go:52: header.Filename used directly without filepath.Base().

### M-04: Distributed admin token empty default
**Status: NOT FIXED**
configs/config.yaml:114: distributed.admin_token is empty.

### M-05: Bridge callback signature default off
**Status: NOT FIXED**
configs/config.yaml.example:60: callback_signature_required defaults to false. Not present in main config.yaml.

### M-06: stringInt manual implementation
**Status: NOT FIXED**
web/middleware_ratelimit.go:195-212: Manual int-to-string instead of strconv.FormatInt(n, 10).

### M-07: Query error leaks internal details
**Status: PARTIALLY FIXED**
web/query_handlers.go:115: handleAPIQuery passes raw err to writeAPIError.
web/query_handlers.go:205: handleQuery uses fmt.Sprintf("Query failed: %v", err) in template data.
sanitizeError() exists in http_helpers.go:285-303 but is not used in these handlers.

### M-08: CORS isOriginAllowed returns true for empty Origin
**Status: NOT FIXED**
web/http_helpers.go:130-131: Returns true when Origin is empty.

### M-09: WebSocket query goroutine may leak
**Status: NOT FIXED**
web/websocket_handlers.go:225-236: No independent timeout context. Uses parent ctx without WithTimeout.

---

## LOW (5)

### L-01: Error messages uppercase
**Status: NOT FIXED**
web/server.go:864: "Query cannot be empty" - still uppercase.
web/server.go:867: "Query is too long..." - still uppercase.

### L-02: CORS duplicate branches
**Status: NOT FIXED**
web/http_helpers.go:251-255: if/else branches execute identical code.

### L-03: generateCSPNonce unsafe fallback
**Status: NOT FIXED**
web/server.go:480-481: Falls back to time.Now().UnixNano() when rand.Read fails.

### L-04: isTrustedRequest not used on all endpoints
**Status: PARTIALLY FIXED**
H-06 fixed the function logic. But many POST endpoints (cookies/import, scheduler/tasks/*, backup/create, import/urls) do not call requireTrustedRequest.

### L-05: map[string]interface{} overuse
**Status: NOT FIXED**
web/ package extensively uses map[string]interface{} for template data and JSON responses. Only apiErrorResponse and apiErrorPayload (http_helpers.go:17-26) are typed structs.

---

## Totals

| Status | Count | Issues |
|--------|-------|--------|
| FIXED | 2 | H-03, H-06 |
| PARTIALLY FIXED | 5 | C-05, M-01, M-07, L-04 (4 items) |
| NOT FIXED | 18 | All others |

## Verdict

**BLOCK** - 4 CRITICAL and 4 HIGH issues remain unfixed.
The review report claimed all 25 were fixed, but actual code shows only 2 fully fixed and 5 partially fixed.

Key files needing changes:
- configs/config.yaml (C-01, H-05, M-04)
- web/server.go (C-02, H-01, L-01, L-03)
- internal/distributed/scheduler.go (C-03)
- web/middleware_ratelimit.go (C-04, M-06)
- web/query_handlers.go (C-05, M-07)
- web/websocket_handlers.go (H-02, M-09)
- internal/alerting/manager.go (H-04)
- web/monitor_handlers.go (M-02, M-03)
- web/http_helpers.go (L-02)
