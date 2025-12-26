#!/bin/bash

# Script to cleanup all tunnels and domain reservations from database
# Useful for development/testing

set -e

# Determine database path
DB_PATH="${1:-./grok.db}"

if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database file not found: $DB_PATH"
    echo "Usage: $0 [database_path]"
    echo "Example: $0 ./grok.db"
    exit 1
fi

echo "🧹 Cleaning up tunnels and domains from database: $DB_PATH"
echo ""

# Show current counts
echo "Current state:"
sqlite3 "$DB_PATH" <<EOF
SELECT 'Tunnels: ' || COUNT(*) FROM tunnels
UNION ALL
SELECT 'Domains: ' || COUNT(*) FROM domains
UNION ALL
SELECT 'Request logs: ' || COUNT(*) FROM request_logs;
EOF

echo ""
read -p "Are you sure you want to delete all tunnels and domains? (y/N) " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cleanup cancelled"
    exit 0
fi

# Perform cleanup
sqlite3 "$DB_PATH" <<EOF
-- Delete all request logs first (foreign key to tunnels)
DELETE FROM request_logs;

-- Delete all tunnels
DELETE FROM tunnels;

-- Delete all domain reservations
DELETE FROM domains;
EOF

echo ""
echo "✅ Cleanup complete!"
echo ""
echo "Final state:"
sqlite3 "$DB_PATH" <<EOF
SELECT 'Tunnels: ' || COUNT(*) FROM tunnels
UNION ALL
SELECT 'Domains: ' || COUNT(*) FROM domains
UNION ALL
SELECT 'Request logs: ' || COUNT(*) FROM request_logs;
EOF

echo ""
echo "✨ Database is now clean. You can create new tunnels with any subdomain."
