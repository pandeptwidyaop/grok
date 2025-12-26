#!/bin/bash

# Test script for realtime stats feature
# This script helps verify that stats (Requests, Data In/Out) update in realtime

echo "==================================="
echo "Realtime Stats Testing Helper"
echo "==================================="
echo ""
echo "This will test the realtime stats feature by:"
echo "1. Making periodic requests through your tunnel"
echo "2. Stats should update every 3 seconds in the dashboard"
echo ""
echo "Prerequisites:"
echo "  - Server running: ./bin/grok-server --config configs/server.yaml"
echo "  - Dashboard running: cd web && npm run dev"
echo "  - Tunnel connected: ./bin/grok http 3000 --name test-stats"
echo ""

# Check if tunnel URL is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <tunnel-public-url>"
    echo "Example: $0 https://abc12345.yourdomain.com"
    exit 1
fi

TUNNEL_URL=$1
REQUEST_COUNT=20
DELAY=2

echo "Testing tunnel: $TUNNEL_URL"
echo "Sending $REQUEST_COUNT requests with ${DELAY}s delay..."
echo ""
echo "Watch your dashboard - stats should update every 3 seconds!"
echo "Check browser console for SSE events: [SSE] Received event: {type: 'tunnel_stats_updated', ...}"
echo ""

for i in $(seq 1 $REQUEST_COUNT); do
    echo "Request $i/$REQUEST_COUNT..."
    curl -s -o /dev/null -w "Status: %{http_code}, Size: %{size_download} bytes, Time: %{time_total}s\n" "$TUNNEL_URL"

    if [ $i -lt $REQUEST_COUNT ]; then
        sleep $DELAY
    fi
done

echo ""
echo "Test complete! Check your dashboard:"
echo "  - Request count should show: $REQUEST_COUNT"
echo "  - Data In/Out should show accumulated bytes"
echo "  - Stats should have updated in realtime (every 3 seconds)"
echo ""
