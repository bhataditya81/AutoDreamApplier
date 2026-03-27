# ==============================================================================
# AutoDreamApplier - Local Dev Stack Launcher (Windows PowerShell)
# Usage: .\scripts\dev.ps1 [-NoBuild] [-NoMigrate] [-NoFrontend] [-Help]
# ==============================================================================
[CmdletBinding()]
param(
    [switch]$NoBuild,
    [switch]$NoMigrate,
    [switch]$NoFrontend,
    [switch]$Help
)

if ($Help) {
    Write-Host "Usage: .\scripts\dev.ps1 [-NoBuild] [-NoMigrate] [-NoFrontend]"
    Write-Host ""
    Write-Host "  -NoBuild      Skip docker-compose --build (use cached images)"
    Write-Host "  -NoMigrate    Skip database migrations"
    Write-Host "  -NoFrontend   Start backend only, skip Next.js"
    exit 0
}

# -- Helpers -------------------------------------------------------------------
function Write-Ok   { param($msg); Write-Host "  [OK]   $msg" -ForegroundColor Green  }
function Write-Warn { param($msg); Write-Host "  [WARN] $msg" -ForegroundColor Yellow }
function Write-Err  { param($msg); Write-Host "  [ERR]  $msg" -ForegroundColor Red    }
function Write-Info { param($msg); Write-Host "  [-->]  $msg" -ForegroundColor Cyan   }

function Write-Header {
    param($msg)
    Write-Host ""
    Write-Host "== $msg ==" -ForegroundColor Blue
}

# -- Banner --------------------------------------------------------------------
Write-Host ""
Write-Host "  +-------------------------------------------+" -ForegroundColor Blue
Write-Host "  |   AutoDreamApplier  --  Dev Stack          |" -ForegroundColor Blue
Write-Host "  +-------------------------------------------+" -ForegroundColor Blue
Write-Host ""

# -- Locate project root -------------------------------------------------------
$ScriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
Set-Location $ProjectRoot

# ==============================================================================
# 1. Prerequisites
# ==============================================================================
Write-Header "Checking prerequisites"

function Assert-Command {
    param($cmd, $hint = "")
    if (Get-Command $cmd -ErrorAction SilentlyContinue) {
        Write-Ok "$cmd found"
    } else {
        $msg = "${cmd} not found."
        if ($hint) { $msg += " $hint" }
        Write-Err $msg
        exit 1
    }
}

Assert-Command "docker" "Install Docker Desktop: https://docs.docker.com/get-docker/"
Assert-Command "node"   "Install Node.js >= 18: https://nodejs.org"
Assert-Command "npm"    "Included with Node.js"

# Resolve docker compose command — prefer v2 plugin, fall back to v1 binary
$DC = $null
$null = docker compose version 2>$null
if ($LASTEXITCODE -eq 0) {
    $DC = "docker compose"
    Write-Ok "docker compose (v2)"
} elseif (Get-Command "docker-compose" -ErrorAction SilentlyContinue) {
    $DC = "docker-compose"
    Write-Ok "docker-compose (v1)"
} else {
    Write-Err "Neither 'docker compose' nor 'docker-compose' found."
    Write-Info "Upgrade Docker Desktop: https://docs.docker.com/compose/install/"
    exit 1
}

$HasMigrate = $false
if (Get-Command "migrate" -ErrorAction SilentlyContinue) {
    Write-Ok "golang-migrate found"
    $HasMigrate = $true
} else {
    Write-Warn "golang-migrate not found - migrations will be skipped"
    Write-Info "Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
}

# ==============================================================================
# 2. Env files
# ==============================================================================
Write-Header "Environment files"

if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
    Write-Ok "Created .env from .env.example"
    Write-Host ""
    Write-Host "  ACTION REQUIRED: Open .env and set ANTHROPIC_API_KEY" -ForegroundColor Yellow
    Write-Host "  AI features (resume tailoring, cover letters) will not work without it." -ForegroundColor Yellow
    Write-Host ""
    Read-Host "  Press Enter to continue, or Ctrl+C to set the key first"
} else {
    Write-Ok ".env already exists"
    $envContent = Get-Content ".env" -Raw
    if ($envContent -match "(?m)^ANTHROPIC_API_KEY=`$") {
        Write-Warn "ANTHROPIC_API_KEY is blank in .env - AI features will fail at runtime"
    }
}

if (-not (Test-Path "frontend\.env.local")) {
    Copy-Item "frontend\.env.local.example" "frontend\.env.local"
    Write-Ok "Created frontend\.env.local"
} else {
    Write-Ok "frontend\.env.local already exists"
}

# ==============================================================================
# 3. Docker Compose
# ==============================================================================
Write-Header "Starting backend (Docker Compose)"

$null = docker info 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Err "Docker daemon is not running."
    Write-Info "Start Docker Desktop, wait for it to fully load, then run this script again."
    exit 1
}

if (-not $NoBuild) {
    Write-Info "Building images and starting containers (first run: 3-5 min)..."
    Invoke-Expression "$DC up --build -d"
} else {
    Write-Info "Starting containers with cached images..."
    Invoke-Expression "$DC up -d"
}

if ($LASTEXITCODE -ne 0) {
    Write-Err "docker compose failed. See output above."
    exit 1
}
Write-Ok "All containers started"

# ==============================================================================
# 4. Wait for PostgreSQL
# ==============================================================================
Write-Header "Waiting for PostgreSQL"

$MaxWait = 90
$Waited  = 0
Write-Host "  Polling postgres" -NoNewline

while ($true) {
    $null = Invoke-Expression "$DC exec -T postgres pg_isready -U autodream -d autodreamapplier" 2>$null
    if ($LASTEXITCODE -eq 0) { break }

    if ($Waited -ge $MaxWait) {
        Write-Host ""
        Write-Err "PostgreSQL not ready after ${MaxWait}s"
        Write-Info "Tip: $DC logs postgres"
        exit 1
    }

    Write-Host "." -NoNewline
    Start-Sleep -Seconds 2
    $Waited += 2
}

Write-Host ""
Write-Ok "PostgreSQL is ready (${Waited}s elapsed)"

# ==============================================================================
# 5. Database migrations
# ==============================================================================
Write-Header "Database migrations"

if ($NoMigrate) {
    Write-Info "Skipped (-NoMigrate flag)"
} elseif ($HasMigrate) {
    & ".\scripts\migrate.sh" up
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "Migrations applied successfully"
    } else {
        Write-Warn "migrate exited non-zero (schema may already be up-to-date)"
    }
} else {
    Write-Warn "Skipped - golang-migrate not installed"
    Write-Info "Run manually: .\scripts\migrate.sh up"
}

# ==============================================================================
# 6. Frontend
# ==============================================================================
Write-Header "Frontend (Next.js)"

if ($NoFrontend) {
    Write-Info "Skipped (-NoFrontend flag)"
    Write-Host ""
    Write-Host "  Backend is fully up!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Service URLs:"
    Write-Host "    API Gateway  -> http://localhost:8080"
    Write-Host "    AI Service   -> http://localhost:8081"
    Write-Host "    MinIO UI     -> http://localhost:9001  (minioadmin / minioadmin)"
    Write-Host ""
    Write-Host "  Stop backend : $DC down" -ForegroundColor Yellow
    Write-Host "  Wipe volumes : $DC down -v" -ForegroundColor Yellow
    exit 0
}

if (-not (Test-Path "frontend\node_modules")) {
    Write-Info "Running npm install (first time - may take ~30s)..."
    npm --prefix frontend install
    Write-Ok "npm install complete"
} else {
    Write-Ok "node_modules present - skipping npm install"
}

# ==============================================================================
# 7. Launch
# ==============================================================================
Write-Host ""
Write-Host "  Backend up! Starting Next.js dev server..." -ForegroundColor Green
Write-Host ""
Write-Host "  Service URLs:"
Write-Host "    Dashboard    -> http://localhost:3000" -ForegroundColor Cyan
Write-Host "    API Gateway  -> http://localhost:8080" -ForegroundColor Cyan
Write-Host "    AI Service   -> http://localhost:8081" -ForegroundColor Cyan
Write-Host "    MinIO UI     -> http://localhost:9001  (minioadmin / minioadmin)" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Ctrl+C stops the frontend. Backend keeps running in Docker." -ForegroundColor Yellow
Write-Host "  Stop everything : $DC down" -ForegroundColor Yellow
Write-Host "  Wipe all data   : $DC down -v" -ForegroundColor Yellow
Write-Host ""

npm --prefix frontend run dev
