#!/usr/bin/env bash
# =============================================================================
# AutoDreamApplier — Local Dev Stack Launcher
# Usage: ./scripts/dev.sh [--no-build] [--no-migrate] [--no-frontend]
# =============================================================================
set -euo pipefail

# ── Flags ─────────────────────────────────────────────────────────────────────
NO_BUILD=false
NO_MIGRATE=false
NO_FRONTEND=false

for arg in "$@"; do
  case $arg in
    --no-build)    NO_BUILD=true ;;
    --no-migrate)  NO_MIGRATE=true ;;
    --no-frontend) NO_FRONTEND=true ;;
    -h|--help)
      echo "Usage: ./scripts/dev.sh [--no-build] [--no-migrate] [--no-frontend]"
      echo ""
      echo "  --no-build     Skip docker-compose --build (use cached images)"
      echo "  --no-migrate   Skip database migrations"
      echo "  --no-frontend  Only start backend, skip Next.js"
      exit 0 ;;
    *) echo "Unknown flag: $arg. Use --help for usage."; exit 1 ;;
  esac
done

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

print_header() { echo -e "\n${BOLD}${BLUE}══ $1 ══${NC}"; }
print_ok()     { echo -e "  ${GREEN}✓${NC} $1"; }
print_warn()   { echo -e "  ${YELLOW}⚠${NC}  $1"; }
print_err()    { echo -e "  ${RED}✗${NC} $1"; }
print_info()   { echo -e "  ${CYAN}→${NC} $1"; }

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${BLUE}"
echo "  ╔═══════════════════════════════════════════╗"
echo "  ║      AutoDreamApplier  ·  Dev Stack        ║"
echo "  ╚═══════════════════════════════════════════╝"
echo -e "${NC}"

# ── Locate project root ───────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

# ── 1. Prerequisites ──────────────────────────────────────────────────────────
print_header "Checking prerequisites"

check_cmd() {
  local cmd="$1"
  local hint="${2:-}"
  if command -v "$cmd" &>/dev/null; then
    print_ok "$cmd"
  else
    print_err "$cmd not found.${hint:+ $hint}"
    exit 1
  fi
}

check_cmd docker "Install Docker Desktop: https://docs.docker.com/get-docker/"
check_cmd node   "Install Node.js ≥ 18: https://nodejs.org"
check_cmd npm    "Comes with Node.js"

# Resolve docker compose command — prefer v2 plugin, fall back to v1 binary
if docker compose version &>/dev/null 2>&1; then
  DC="docker compose"
  print_ok "docker compose (v2)"
elif command -v docker-compose &>/dev/null; then
  DC="docker-compose"
  print_ok "docker-compose (v1)"
else
  print_err "Neither 'docker compose' nor 'docker-compose' found."
  print_info "Upgrade Docker Desktop to get the v2 plugin: https://docs.docker.com/compose/install/"
  exit 1
fi

if command -v migrate &>/dev/null; then
  print_ok "golang-migrate"
  HAS_MIGRATE=true
else
  print_warn "golang-migrate not found — migrations will be skipped"
  print_info "Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
  HAS_MIGRATE=false
fi

# ── 2. Env files ──────────────────────────────────────────────────────────────
print_header "Environment files"

if [ ! -f .env ]; then
  cp .env.example .env
  print_ok "Created .env from .env.example"
  echo ""
  echo -e "  ${YELLOW}${BOLD}ACTION REQUIRED:${NC} ${YELLOW}Open .env and set ANTHROPIC_API_KEY${NC}"
  echo -e "  ${YELLOW}AI features (resume tailoring, cover letters) won't work without it.${NC}"
  echo ""
  read -rp "  Press Enter to continue, or Ctrl+C to set the key first... "
else
  print_ok ".env already exists"
  # Warn if key is still blank
  if grep -qE '^ANTHROPIC_API_KEY=$' .env 2>/dev/null; then
    print_warn "ANTHROPIC_API_KEY is not set in .env — AI features will fail"
  fi
fi

if [ ! -f frontend/.env.local ]; then
  cp frontend/.env.local.example frontend/.env.local
  print_ok "Created frontend/.env.local"
else
  print_ok "frontend/.env.local already exists"
fi

# ── 3. Docker Compose ─────────────────────────────────────────────────────────
print_header "Starting backend (Docker Compose)"

# Verify Docker daemon
if ! docker info &>/dev/null 2>&1; then
  print_err "Docker daemon is not running. Start Docker Desktop and try again."
  exit 1
fi

COMPOSE_ARGS="-d"
if [ "${NO_BUILD}" = false ]; then
  COMPOSE_ARGS="--build ${COMPOSE_ARGS}"
  print_info "Building images and starting containers (first run takes 3-5 min)..."
else
  print_info "Starting containers with cached images..."
fi

${DC} up ${COMPOSE_ARGS}
print_ok "All containers started"

# ── 4. Wait for PostgreSQL ────────────────────────────────────────────────────
print_header "Waiting for PostgreSQL"

MAX_WAIT=90
WAITED=0
printf "  Polling"
until ${DC} exec -T postgres pg_isready -U autodream -d autodreamapplier &>/dev/null 2>&1; do
  if [ "${WAITED}" -ge "${MAX_WAIT}" ]; then
    echo ""
    print_err "PostgreSQL not ready after ${MAX_WAIT}s"
    print_info "Check logs: ${DC} logs postgres"
    exit 1
  fi
  printf "."
  sleep 2
  WAITED=$((WAITED + 2))
done
echo ""
print_ok "PostgreSQL is ready (after ${WAITED}s)"

# ── 5. Database migrations ────────────────────────────────────────────────────
print_header "Database migrations"

if [ "${NO_MIGRATE}" = true ]; then
  print_info "Skipped (--no-migrate)"
elif [ "${HAS_MIGRATE}" = true ]; then
  ./scripts/migrate.sh up
  print_ok "Migrations applied"
else
  print_warn "Skipped — install golang-migrate to run automatically"
  print_info "Manual: ./scripts/migrate.sh up"
fi

# ── 6. Frontend ───────────────────────────────────────────────────────────────
print_header "Frontend (Next.js)"

if [ "${NO_FRONTEND}" = true ]; then
  print_info "Skipped (--no-frontend)"
  echo ""
  echo -e "  ${BOLD}${GREEN}✅ Backend is fully up!${NC}"
  echo ""
  echo -e "  ${BOLD}Service URLs:${NC}"
  echo -e "  ${CYAN}→ API Gateway${NC}   http://localhost:8080"
  echo -e "  ${CYAN}→ AI Service${NC}    http://localhost:8081"
  echo -e "  ${CYAN}→ MinIO Console${NC} http://localhost:9001  (minioadmin / minioadmin)"
  echo ""
  echo -e "  ${YELLOW}To stop: ${DC} down${NC}"
  exit 0
fi

if [ ! -d frontend/node_modules ]; then
  print_info "Running npm install (first time — may take ~30s)..."
  npm --prefix frontend install
  print_ok "Dependencies installed"
else
  print_ok "node_modules already present (skipping install)"
fi

# ── 7. Summary & launch ───────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}  ✅ Backend is up! Launching Next.js dev server...${NC}"
echo ""
echo -e "  ${BOLD}Service URLs:${NC}"
echo -e "  ${CYAN}→ Dashboard     ${NC}${BOLD}http://localhost:3000${NC}"
echo -e "  ${CYAN}→ API Gateway   ${NC}http://localhost:8080"
echo -e "  ${CYAN}→ AI Service    ${NC}http://localhost:8081"
echo -e "  ${CYAN}→ MinIO Console ${NC}http://localhost:9001  ${YELLOW}(minioadmin / minioadmin)${NC}"
echo ""
echo -e "  ${YELLOW}Ctrl+C stops the frontend. Backend keeps running.${NC}"
echo -e "  ${YELLOW}To stop everything: ${DC} down${NC}"
echo -e "  ${YELLOW}To wipe all data:   ${DC} down -v${NC}"
echo ""

# Launch Next.js in the foreground so hot-reload works and logs are visible
npm --prefix frontend run dev
