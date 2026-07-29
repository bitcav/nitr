#requires -Version 7.0
# arm64_endpoint_probe.ps1 — evidence gathering for the windows/arm64
# build-only investigation. NOT a pass/fail test of endpoint contents:
# an endpoint that returns garbage on Windows on Arm is a FINDING and goes
# in the report; it must not fail the CI job.
#
# Exit codes:
#   0 — probe ran; report written (endpoint results may be good or bad)
#   1 — genuine failure: binary won't run (init()-panic class), server
#       won't start, or the API key can't be obtained
#
# Lifecycle note: the server holds nitr.db's flock for its whole lifetime,
# and `nitr key` needs the db. So: start server (provisions db + random key)
# -> stop server (releases flock) -> `nitr key` -> start server again -> probe.
param(
    [string]$Exe = "./nitr_windows_arm64.exe",
    [string]$ReportPath = "arm64-endpoint-report.md",
    [int]$Port = 18765
)

$ErrorActionPreference = "Continue"
$endpoints = @('/', '/cpu', '/bios', '/bandwidth', '/chassis', '/disks',
    '/drives', '/devices', '/gpu', '/host', '/isp', '/network', '/processes',
    '/ram', '/baseboard', '/product', '/memory', '/swap', '/loadavg', '/sensors')

$script:rows = [System.Collections.Generic.List[string]]::new()
$script:versionOut = "(not run)"

function Add-Row([string]$Ep, [string]$Http, [string]$Result, [string]$Note) {
    $n = ($Note -replace '\|', '\|' -replace "`r?`n", ' ')
    if ($n.Length -gt 120) { $n = $n.Substring(0, 120) + "..." }
    $script:rows.Add("| $Ep | $Http | $Result | $n |")
}

function Write-Report([string]$Verdict) {
    $lines = @(
        "# nitr windows/arm64 endpoint probe",
        "",
        "- Runner: $env:RUNNER_OS/$env:RUNNER_ARCH ($env:RUNNER_NAME)",
        "- Version string: ``$($script:versionOut.Trim())``",
        "- Verdict: $Verdict",
        ""
    )
    if ($script:rows.Count -gt 0) {
        $lines += "| Endpoint | HTTP | Result | Note |"
        $lines += "|----------|------|--------|------|"
        $lines += $script:rows
    }
    $text = $lines -join "`n"
    Set-Content -Path $ReportPath -Value $text -Encoding utf8
    Write-Host $text
}

function Start-NitrServer {
    $out = Join-Path $script:dataDir "server.out.log"
    $err = Join-Path $script:dataDir "server.err.log"
    $script:server = Start-Process -FilePath $script:exePath -PassThru -NoNewWindow `
        -ArgumentList @("server", "--host", "127.0.0.1", "--port", "$Port", "--data-dir", $script:dataDir) `
        -RedirectStandardOutput $out -RedirectStandardError $err
}

function Stop-NitrServer {
    if ($script:server -and -not $script:server.HasExited) {
        Stop-Process -Id $script:server.Id -Force -ErrorAction SilentlyContinue
        $script:server.WaitForExit()
    }
}

function Wait-Ready([int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:${Port}/ready" -TimeoutSec 5
            if ($r.StatusCode -eq 200) { return $true }
        } catch { Start-Sleep -Milliseconds 500 }
    }
    return $false
}

# --- 1. Smoke test gate -----------------------------------------------------
$script:exePath = (Resolve-Path $Exe).Path
$script:versionOut = (& $script:exePath version 2>&1 | Out-String)
if ($script:versionOut -notmatch "Nitr v") {
    Write-Host "SMOKE TEST FAILED: binary does not print a version string. Output was:"
    Write-Host $script:versionOut
    Write-Report "FAIL — binary does not run on windows/arm64 (init()-panic class). Endpoint table moot."
    exit 1
}
Write-Host "Smoke test OK: $($script:versionOut.Trim())"

# --- 2. Server lifecycle ----------------------------------------------------
$env:NITR_OPEN_BROWSER_ON_STARTUP = "false"
$script:dataDir = Join-Path ([IO.Path]::GetTempPath()) ("nitr-arm-probe-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Path $script:dataDir | Out-Null

try {
    Start-NitrServer
    if (-not (Wait-Ready 60)) {
        Write-Report "FAIL — server did not become ready within 60s"
        exit 1
    }
    Write-Host "Server start #1 OK (db provisioned)"
    Stop-NitrServer  # releases the nitr.db flock so `nitr key` can open it

    $keyOut = ("123456" | & $script:exePath key --data-dir $script:dataDir 2>&1 | Out-String)
    if ($keyOut -notmatch "api key is:\s*(\S+)") {
        Write-Host "Could not obtain API key. Output was:"
        Write-Host $keyOut
        Write-Report "FAIL — could not obtain API key via ``nitr key``"
        exit 1
    }
    $apiKey = $Matches[1]
    Write-Host "API key obtained"

    Start-NitrServer
    if (-not (Wait-Ready 60)) {
        Write-Report "FAIL — server did not become ready on second start"
        exit 1
    }
    Write-Host "Server start #2 OK, probing endpoints"

    # --- 3. Probe (results are data, never failures) -------------------------
    foreach ($ep in $endpoints) {
        $uri = "http://127.0.0.1:${Port}/api/v1$ep"
        try {
            $r = Invoke-WebRequest -Uri $uri -Headers @{ "x-api-key" = $apiKey } -TimeoutSec 30
            $t = ("$($r.Content)").Trim()
            if ($t.Length -eq 0) {
                Add-Row $ep "$($r.StatusCode)" "empty body" ""
            } elseif ($t -in @("null", "{}", "[]", '""')) {
                Add-Row $ep "$($r.StatusCode)" "empty ($t)" ""
            } else {
                try {
                    $j = $t | ConvertFrom-Json -ErrorAction Stop
                    $n = if ($j -is [array]) { $j.Count } else { @($j.PSObject.Properties).Count }
                    if ($n -eq 0) { Add-Row $ep "$($r.StatusCode)" "empty JSON" "" }
                    else { Add-Row $ep "$($r.StatusCode)" "populated ($n)" "" }
                } catch {
                    Add-Row $ep "$($r.StatusCode)" "non-JSON body" $t
                }
            }
        } catch {
            $resp = $_.Exception.Response
            if ($resp) { Add-Row $ep "$([int]$resp.StatusCode)" "HTTP error" $_.Exception.Message }
            else { Add-Row $ep "—" "request failed" $_.Exception.Message }
        }
    }
    Write-Report "probe completed — see table; bad cells are findings, not CI failures"
    exit 0
} finally {
    Stop-NitrServer
}
