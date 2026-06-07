param(
    [string]$BaseUrl = "http://127.0.0.1:8448",
    [string]$AuthToken = "",
    [string]$BatchId = "bridge_e2e_full",
    [switch]$StrictSignature,
    [switch]$RotateToken
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Msg)
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Write-Host "[$ts] $Msg"
}

function Invoke-Authed {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [string]$ContentType = "",
        [string]$Body = ""
    )
    $allHeaders = @{}
    if (-not [string]::IsNullOrWhiteSpace($AuthToken)) {
        $allHeaders["Authorization"] = "Bearer $AuthToken"
    }
    foreach ($k in $Headers.Keys) { $allHeaders[$k] = $Headers[$k] }
    $params = @{ Method = $Method; Uri = $Uri; Headers = $allHeaders }
    if ($ContentType) { $params["ContentType"] = $ContentType }
    if ($Body) { $params["Body"] = $Body }
    return Invoke-RestMethod @params
}

function New-BridgeSignatureHeaders {
    param(
        [Parameter(Mandatory = $true)][string]$Token,
        [Parameter(Mandatory = $true)][string]$Body
    )

    $timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds().ToString()
    $nonce = [Guid]::NewGuid().ToString("N")

    $bodyBytes = [Text.Encoding]::UTF8.GetBytes($Body)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bodyHashBytes = $sha.ComputeHash($bodyBytes)
    } finally {
        $sha.Dispose()
    }
    $bodyHashHex = ([BitConverter]::ToString($bodyHashBytes)).Replace("-", "").ToLowerInvariant()

    $canonical = "$timestamp`n$nonce`n$bodyHashHex"
    $hmac = [System.Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes($Token))
    try {
        $sigBytes = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($canonical))
    } finally {
        $hmac.Dispose()
    }
    $sigHex = ([BitConverter]::ToString($sigBytes)).Replace("-", "").ToLowerInvariant()

    return @{
        "X-Bridge-Timestamp" = $timestamp
        "X-Bridge-Nonce" = $nonce
        "X-Bridge-Signature" = $sigHex
    }
}

$passCount = 0
$failCount = 0

function Assert-NotTrue {
    param([bool]$Condition, [string]$Label)
    if ($Condition) {
        Write-Step "[PASS] $Label"
        $script:passCount++
    } else {
        Write-Step "[FAIL] $Label"
        $script:failCount++
    }
}

# ============================================================
# Phase 1: Pairing
# ============================================================
Write-Step "[1/7] Pairing for bridge token..."
$pairReq = @{ client_id = "powershell-e2e"; pair_code = "dev-pair" } | ConvertTo-Json
$pairResp = Invoke-Authed -Method Post -Uri "$BaseUrl/api/v1/screenshot/bridge/pair" -ContentType "application/json" -Body $pairReq
$token = $pairResp.token
Assert-NotTrue -Condition (![string]::IsNullOrWhiteSpace($token)) -Label "Pairing returned a non-empty token"
if ([string]::IsNullOrWhiteSpace($token)) { throw "Pairing failed: empty token" }

$authHeader = @{ Authorization = "Bearer $token" }
Write-Step "[1/7] Token acquired: $($token.Substring(0, [Math]::Min(16, $token.Length)))..."

# ============================================================
# Phase 1.5: Token rotation (optional)
# ============================================================
if ($RotateToken) {
    Write-Step "[1.5/7] Rotate bridge token..."
    $rotateReq = @{ revoke_old = $true } | ConvertTo-Json
    $rotateResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/screenshot/bridge/token/rotate" -Headers $authHeader -ContentType "application/json" -Body $rotateReq
    $newToken = $rotateResp.token
    Assert-NotTrue -Condition (![string]::IsNullOrWhiteSpace($newToken)) -Label "Token rotation returned a new token"
    if ([string]::IsNullOrWhiteSpace($newToken)) { throw "Rotate token failed: empty token" }
    $token = $newToken
    $authHeader = @{ Authorization = "Bearer $token" }
    Write-Step "[1.5/7] Token rotated successfully"
}

# ============================================================
# Phase 2: Start async batch screenshot (generates tasks)
# ============================================================
Write-Step "[2/7] Start async batch screenshot request (screenshot action)..."
$batchBody = @{ urls = @("https://example.com"); batch_id = $BatchId; concurrency = 1 } | ConvertTo-Json -Depth 5
$batchJob = Start-Job -ScriptBlock {
    param($url, $body)
    Invoke-RestMethod -Method Post -Uri "$url/api/v1/screenshot/batch-urls" -ContentType "application/json" -Body $body
} -ArgumentList $BaseUrl, $batchBody

Start-Sleep -Seconds 1

# ============================================================
# Phase 3: Pull bridge task (screenshot action)
# ============================================================
Write-Step "[3/7] Pull first bridge task (screenshot action)..."
$taskResp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/v1/screenshot/bridge/tasks/next" -Headers $authHeader
Assert-NotTrue -Condition ($null -ne $taskResp.task) -Label "Got a screenshot task from bridge queue"
if ($null -eq $taskResp.task) { throw "No task available from bridge queue" }
$screenshotTask = $taskResp.task
Write-Step "[3/7] Task action: $(if ($screenshotTask.action) { $screenshotTask.action } else { '(default=screenshot)' })"

# ============================================================
# Phase 3b: Start another batch to generate an open task
# ============================================================
Write-Step "[3b/7] Start async open-task batch request..."
$openBatchBody = @{ urls = @("https://fofa.info"); batch_id = "${BatchId}_open"; concurrency = 1 } | ConvertTo-Json -Depth 5
$openBatchJob = Start-Job -ScriptBlock {
    param($url, $body)
    Invoke-RestMethod -Method Post -Uri "$url/api/v1/screenshot/batch-urls" -ContentType "application/json" -Body $body
} -ArgumentList $BaseUrl, $openBatchBody

Start-Sleep -Seconds 1

# ============================================================
# Phase 3c: Pull second bridge task
# ============================================================
Write-Step "[3c/7] Pull second bridge task..."
$taskResp2 = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/v1/screenshot/bridge/tasks/next" -Headers $authHeader
Assert-NotTrue -Condition ($null -ne $taskResp2.task) -Label "Got a second task from bridge queue"
if ($null -ne $taskResp2.task) {
    $secondTask = $taskResp2.task
    Write-Step "[3c/7] Second task action: $(if ($secondTask.action) { $secondTask.action } else { '(default=screenshot)' })"
}

# ============================================================
# Phase 4: Push mock results
# ============================================================
$imageData = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII="

# Result for first task
Write-Step "[4/7] Push mock result for first task (screenshot)..."
$mockReq = @{
    request_id = $screenshotTask.request_id
    success = $true
    image_path = ""
    image_data = $imageData
    batch_id = $screenshotTask.batch_id
    url = $screenshotTask.url
    error_code = ""
    error = ""
} | ConvertTo-Json -Depth 6

$mockHeaders = @{ Authorization = "Bearer $token" }
if ($StrictSignature) {
    $sigHeaders = New-BridgeSignatureHeaders -Token $token -Body $mockReq
    foreach ($key in $sigHeaders.Keys) {
        $mockHeaders[$key] = $sigHeaders[$key]
    }
}

$mockResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/screenshot/bridge/mock/result" -Headers $mockHeaders -ContentType "application/json" -Body $mockReq
Assert-NotTrue -Condition ($mockResp.success -eq $true) -Label "First task mock result accepted"

# Result for second task (if available)
if ($null -ne $taskResp2.task) {
    Write-Step "[4/7] Push mock result for second task..."
    $mockReq2 = @{
        request_id = $secondTask.request_id
        success = $true
        image_path = ""
        image_data = $imageData
        batch_id = $secondTask.batch_id
        url = $secondTask.url
        error_code = ""
        error = ""
    } | ConvertTo-Json -Depth 6

    $mockResp2 = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/screenshot/bridge/mock/result" -Headers $mockHeaders -ContentType "application/json" -Body $mockReq2
    Assert-NotTrue -Condition ($mockResp2.success -eq $true) -Label "Second task mock result accepted"
}

# ============================================================
# Phase 5: Wait for batch API completion
# ============================================================
Write-Step "[5/7] Wait for batch API completion..."
$batchResp = Receive-Job -Job $batchJob -Wait -AutoRemoveJob
Assert-NotTrue -Condition ($null -ne $batchResp) -Label "Batch screenshot job completed"

if ($null -ne $taskResp2.task) {
    $openBatchResp = Receive-Job -Job $openBatchJob -Wait -AutoRemoveJob
    Assert-NotTrue -Condition ($null -ne $openBatchResp) -Label "Open-task batch screenshot job completed"
}

# ============================================================
# Phase 6: Query bridge status — verify timestamps
# ============================================================
Write-Step "[6/7] Query bridge status and verify observability fields..."
$statusResp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/v1/screenshot/bridge/status" -Headers $authHeader

# Verify new timestamp fields exist and are non-zero
Assert-NotTrue -Condition ($null -ne $statusResp.last_pair_at -and $statusResp.last_pair_at -gt 0) -Label "last_pair_at is present and non-zero"
Assert-NotTrue -Condition ($null -ne $statusResp.last_task_pull_at -and $statusResp.last_task_pull_at -gt 0) -Label "last_task_pull_at is present and non-zero"
Assert-NotTrue -Condition ($null -ne $statusResp.last_callback_at -and $statusResp.last_callback_at -gt 0) -Label "last_callback_at is present and non-zero"

# Verify router fields
Assert-NotTrue -Condition ($null -ne $statusResp.router_mode) -Label "router_mode is present"
Assert-NotTrue -Condition ($null -ne $statusResp.router_cdp_healthy) -Label "router_cdp_healthy is present"
Assert-NotTrue -Condition ($null -ne $statusResp.router_ext_healthy) -Label "router_ext_healthy is present"

# Verify timestamp ordering: pair <= task_pull <= callback
if ($statusResp.last_pair_at -gt 0 -and $statusResp.last_task_pull_at -gt 0) {
    Assert-NotTrue -Condition ($statusResp.last_task_pull_at -ge $statusResp.last_pair_at) -Label "Timestamps ordered: last_pair_at <= last_task_pull_at"
}
if ($statusResp.last_task_pull_at -gt 0 -and $statusResp.last_callback_at -gt 0) {
    Assert-NotTrue -Condition ($statusResp.last_callback_at -ge $statusResp.last_task_pull_at) -Label "Timestamps ordered: last_task_pull_at <= last_callback_at"
}

# ============================================================
# Phase 7: Summary
# ============================================================
Write-Host ""
Write-Host "============================================================"
Write-Host "Bridge E2E Test Summary"
Write-Host "============================================================"
Write-Host "Passed: $passCount"
Write-Host "Failed: $failCount"
Write-Host ""

if ($failCount -gt 0) {
    Write-Host "E2E validation FAILED. Check logs above for details." -ForegroundColor Red
    exit 1
} else {
    Write-Host "E2E validation PASSED. All checks green." -ForegroundColor Green
    exit 0
}
