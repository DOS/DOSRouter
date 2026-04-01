# ClawRouter Update Script for Windows (PowerShell)
# Usage: iwr -useb https://blockrun.ai/ClawRouter-update.ps1 | iex
#    or: powershell -ExecutionPolicy Bypass -Command "iwr -useb https://blockrun.ai/ClawRouter-update.ps1 | iex"
#
# Run as regular user (no admin needed)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$PLUGIN_DIR = "$env:USERPROFILE\.openclaw\extensions\clawrouter"
$CONFIG_PATH = "$env:USERPROFILE\.openclaw\openclaw.json"
$WALLET_FILE = "$env:USERPROFILE\.openclaw\blockrun\wallet.key"

function Write-Ok  { param($msg) Write-Host "  $([char]0x2713) $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  ! $msg" -ForegroundColor Yellow }
function Write-Err  { param($msg) Write-Host "  x $msg" -ForegroundColor Red }
function Write-Step { param($msg) Write-Host "`n-> $msg" }

Write-Host ""
Write-Host "ClawRouter Update (Windows)" -ForegroundColor Cyan
Write-Host ""

# ── Step 1: Back up wallet ────────────────────────────────────
Write-Step "Backing up wallet..."
$walletBackup = $null
if (Test-Path $WALLET_FILE) {
    $walletKey = (Get-Content $WALLET_FILE -Raw).Trim()
    if ($walletKey -match '^0x[0-9a-fA-F]{64}$') {
        $walletBackup = "$WALLET_FILE.bak.$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
        Copy-Item $WALLET_FILE $walletBackup
        Write-Ok "Wallet backed up: $walletBackup"
    } else {
        Write-Warn "Wallet file found but has unexpected format — skipping backup"
    }
} else {
    Write-Host "  i No existing wallet found"
}

# ── Step 2: Stop old proxy ────────────────────────────────────
Write-Step "Stopping old proxy..."
try {
    $procs = Get-NetTCPConnection -LocalPort 8402 -ErrorAction SilentlyContinue |
             Select-Object -ExpandProperty OwningProcess -Unique
    foreach ($pid in $procs) {
        if ($pid -gt 0) { Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue }
    }
    Write-Ok "Port 8402 cleared"
} catch {
    Write-Host "  i Could not check port 8402 (normal if proxy was not running)"
}

# ── Step 3: Clean stale config entries ────────────────────────
Write-Step "Cleaning config..."
if (Test-Path $CONFIG_PATH) {
    try {
        $cfg = Get-Content $CONFIG_PATH -Raw | ConvertFrom-Json
        $changed = $false
        if ($cfg.plugins -and $cfg.plugins.entries -and $cfg.plugins.entries.PSObject.Properties['clawrouter']) {
            $cfg.plugins.entries.PSObject.Properties.Remove('clawrouter')
            $changed = $true
        }
        if ($cfg.plugins -and $cfg.plugins.installs -and $cfg.plugins.installs.PSObject.Properties['clawrouter']) {
            $cfg.plugins.installs.PSObject.Properties.Remove('clawrouter')
            $changed = $true
        }
        if ($changed) {
            $cfg | ConvertTo-Json -Depth 20 | Set-Content $CONFIG_PATH -Encoding UTF8
            Write-Ok "Removed stale plugin entries"
        } else {
            Write-Ok "Config already clean"
        }
    } catch {
        Write-Warn "Could not parse config: $_"
    }
}

# ── Step 3b: Ensure baseUrl / apiKey ──────────────────────────
Write-Step "Verifying provider config..."
if (Test-Path $CONFIG_PATH) {
    try {
        $cfg = Get-Content $CONFIG_PATH -Raw | ConvertFrom-Json
        $provider = $null
        if ($cfg.models -and $cfg.models.providers) { $provider = $cfg.models.providers.blockrun }
        if ($provider) {
            $changed = $false
            if (-not $provider.baseUrl) { $provider | Add-Member -NotePropertyName baseUrl -NotePropertyValue 'http://127.0.0.1:8402/v1' -Force; $changed = $true; Write-Ok "Fixed missing baseUrl" }
            if (-not $provider.apiKey)  { $provider | Add-Member -NotePropertyName apiKey  -NotePropertyValue 'x402-proxy-handles-auth' -Force; $changed = $true; Write-Ok "Fixed missing apiKey" }
            if ($changed) { $cfg | ConvertTo-Json -Depth 20 | Set-Content $CONFIG_PATH -Encoding UTF8 }
            else { Write-Ok "Provider config OK" }
        }
    } catch {
        Write-Warn "Could not verify provider config: $_"
    }
}

# ── Step 4: Get latest version from npm ───────────────────────
Write-Step "Fetching latest version from npm..."
try {
    $verFile = Join-Path $env:TEMP "clawrouter-latest-ver.txt"
    & cmd /c "npm view @blockrun/clawrouter@latest version > `"$verFile`" 2>nul"
    $LATEST_VERSION = (Get-Content $verFile -ErrorAction Stop).Trim()
    Remove-Item $verFile -ErrorAction SilentlyContinue
    if (-not $LATEST_VERSION -or $LATEST_VERSION -match 'error|ERR') {
        throw "npm view returned: $LATEST_VERSION"
    }
    Write-Ok "Latest: v$LATEST_VERSION"
} catch {
    Write-Err "Cannot determine latest version: $_"
    Write-Host "  Check npm is installed and you have internet access."
    exit 1
}

# ── Step 5: Install directly from npm (bypasses openclaw cache) ───
Write-Step "Downloading ClawRouter v$LATEST_VERSION from npm..."

$tmpDir = Join-Path $env:TEMP "clawrouter-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    # Run npm pack via cmd.exe to completely bypass PowerShell's stderr-as-error behavior
    & cmd /c "npm pack `"@blockrun/clawrouter@$LATEST_VERSION`" --pack-destination `"$tmpDir`" --prefer-online >nul 2>nul"

    $tarball = Get-ChildItem "$tmpDir\*.tgz" | Select-Object -First 1
    if (-not $tarball) { throw "npm pack produced no tarball in $tmpDir" }

    # Remove old install
    if (Test-Path $PLUGIN_DIR) {
        Remove-Item -Recurse -Force $PLUGIN_DIR
    }
    New-Item -ItemType Directory -Path $PLUGIN_DIR -Force | Out-Null

    # Extract (tar is built into Windows 10+ / Server 2019+)
    tar -xzf $tarball.FullName -C $PLUGIN_DIR --strip-components=1
    if ($LASTEXITCODE -ne 0) { throw "tar extraction failed (exit $LASTEXITCODE)" }

    Write-Ok "Extracted v$LATEST_VERSION to $PLUGIN_DIR"
} finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}

# ── Step 5b: Install npm dependencies ─────────────────────────
Write-Step "Installing dependencies (Solana, x402, etc.)..."
$logFile = Join-Path $env:TEMP "clawrouter-npm-install.log"
& cmd /c "cd /d `"$PLUGIN_DIR`" && npm install --omit=dev > `"$logFile`" 2>&1"
if ($LASTEXITCODE -ne 0) {
    Write-Err "npm install failed. Log: $logFile"
    Get-Content $logFile -ErrorAction SilentlyContinue | Select-Object -Last 20 | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    exit 1
}
Write-Ok "Dependencies installed"

# ── Step 6: Register plugin in openclaw config ────────────────
Write-Step "Registering plugin..."
if (Test-Path $CONFIG_PATH) {
    try {
        $cfg = Get-Content $CONFIG_PATH -Raw | ConvertFrom-Json
        if (-not $cfg.plugins) { $cfg | Add-Member -NotePropertyName plugins -NotePropertyValue ([PSCustomObject]@{}) -Force }
        if (-not $cfg.plugins.allow) { $cfg.plugins | Add-Member -NotePropertyName allow -NotePropertyValue @() -Force }
        $allow = [System.Collections.Generic.List[string]]$cfg.plugins.allow
        if (-not $allow.Contains('clawrouter')) {
            $allow.Add('clawrouter')
            $cfg.plugins.allow = $allow.ToArray()
            $cfg | ConvertTo-Json -Depth 20 | Set-Content $CONFIG_PATH -Encoding UTF8
            Write-Ok "Added clawrouter to plugins.allow"
        } else {
            Write-Ok "Plugin already in allow list"
        }
    } catch {
        Write-Warn "Could not update plugins.allow: $_"
    }
}

# ── Step 7: Inject auth profile ───────────────────────────────
Write-Step "Setting up auth profile..."
$authDir  = "$env:USERPROFILE\.openclaw\agents\main\agent"
$authPath = "$authDir\auth-profiles.json"
New-Item -ItemType Directory -Path $authDir -Force | Out-Null
$store = [PSCustomObject]@{ version = 1; profiles = [PSCustomObject]@{} }
if (Test-Path $authPath) {
    try { $store = Get-Content $authPath -Raw | ConvertFrom-Json } catch {}
}
if (-not $store.profiles.PSObject.Properties['blockrun:default']) {
    $store.profiles | Add-Member -NotePropertyName 'blockrun:default' `
        -NotePropertyValue ([PSCustomObject]@{ type = 'api_key'; provider = 'blockrun'; key = 'x402-proxy-handles-auth' }) -Force
    $store | ConvertTo-Json -Depth 10 | Set-Content $authPath -Encoding UTF8
    Write-Ok "Auth profile created"
} else {
    Write-Ok "Auth profile already exists"
}

# ── Step 8: Clean models cache ────────────────────────────────
Write-Step "Cleaning models cache..."
Get-ChildItem "$env:USERPROFILE\.openclaw\agents\*\agent\models.json" -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
Write-Ok "Models cache cleared"

# ── Step 9: Verify wallet survived ────────────────────────────
Write-Step "Verifying wallet integrity..."
if (Test-Path $WALLET_FILE) {
    $key = (Get-Content $WALLET_FILE -Raw).Trim()
    if ($key -match '^0x[0-9a-fA-F]{64}$') {
        Write-Ok "Wallet key intact"
    } else {
        if ($walletBackup -and (Test-Path $walletBackup)) {
            Copy-Item $walletBackup $WALLET_FILE -Force
            Write-Ok "Wallet restored from backup"
        } else {
            Write-Warn "Wallet file may be corrupted and no backup found"
        }
    }
} else {
    if ($walletBackup -and (Test-Path $walletBackup)) {
        New-Item -ItemType Directory -Path (Split-Path $WALLET_FILE) -Force | Out-Null
        Copy-Item $walletBackup $WALLET_FILE -Force
        Write-Ok "Wallet restored from backup"
    } else {
        Write-Host "  i No wallet found — a new one will be generated on first start"
    }
}

# ── Done ──────────────────────────────────────────────────────
Write-Host ""
Write-Host "ClawRouter v$LATEST_VERSION installed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "  Run: openclaw gateway restart" -ForegroundColor Cyan
Write-Host ""
Write-Host "  To verify: npx @blockrun/clawrouter doctor"
Write-Host ""
