#!/bin/bash
# Database migration script
# Usage: ./scripts/migrate.sh [up|down|version|force VERSION]

set -euo pipefail

# Load .env if it exists
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

# Default values
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-autodreamapplier}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
MIGRATIONS_DIR="./migrations"

ACTION="${1:-up}"

echo "🗄️  AutoDreamApplier Database Migration"
echo "   Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}"
echo "   Action: ${ACTION}"
echo ""

# Check if migrate tool is installed
if ! command -v migrate &> /dev/null; then
    echo "❌ 'migrate' tool not found. Install it:"
    echo "   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
    echo ""
    echo "   Or on macOS: brew install golang-migrate"
    exit 1
fi

case "${ACTION}" in
    up)
        echo "⬆️  Running migrations UP..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" up
        echo "✅ Migrations applied successfully"
        ;;
    down)
        echo "⬇️  Running migrations DOWN (rolling back last migration)..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" down 1
        echo "✅ Rollback complete"
        ;;
    down-all)
        echo "⬇️  Running ALL migrations DOWN..."
        echo "⚠️  WARNING: This will drop all tables!"
        read -p "Are you sure? (y/N): " confirm
        if [[ "${confirm}" =~ ^[Yy]$ ]]; then
            migrate -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" down -all
            echo "✅ All migrations rolled back"
        else
            echo "Cancelled."
        fi
        ;;
    version)
        echo "📋 Current migration version:"
        migrate -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" version
        ;;
    force)
        VERSION="${2:-}"
        if [ -z "${VERSION}" ]; then
            echo "❌ Usage: ./scripts/migrate.sh force VERSION"
            exit 1
        fi
        echo "🔧 Forcing migration version to ${VERSION}..."
        migrate -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" force "${VERSION}"
        echo "✅ Version forced to ${VERSION}"
        ;;
    create)
        NAME="${2:-}"
        if [ -z "${NAME}" ]; then
            echo "❌ Usage: ./scripts/migrate.sh create MIGRATION_NAME"
            exit 1
        fi
        echo "📝 Creating new migration: ${NAME}"
        migrate create -ext sql -dir "${MIGRATIONS_DIR}" -seq "${NAME}"
        echo "✅ Migration files created"
        ;;
    *)
        echo "Usage: ./scripts/migrate.sh [up|down|down-all|version|force VERSION|create NAME]"
        exit 1
        ;;
esac
