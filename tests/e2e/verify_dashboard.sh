#!/bin/bash

# E2E Test: Verify Dashboard Data Display
# This script tests that tunnel data is correctly displayed in the dashboard

set -e

echo "=== Grok Dashboard Verification Test ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
SERVER_URL="http://localhost:4040"
API_URL="$SERVER_URL/api"

echo "1. Testing server health..."
if curl -s "$SERVER_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Server is running"
else
    echo -e "${RED}✗${NC} Server is not running. Please start the server first:"
    echo "   ./bin/grok-server"
    exit 1
fi

echo ""
echo "2. Testing login..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}')

TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | sed 's/"token":"//')

if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗${NC} Login failed"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓${NC} Login successful"
echo "   Token: ${TOKEN:0:20}..."

echo ""
echo "3. Fetching active tunnels..."
TUNNELS_RESPONSE=$(curl -s "$API_URL/tunnels" \
    -H "Authorization: Bearer $TOKEN")

echo "   Raw response:"
echo "   $TUNNELS_RESPONSE" | head -c 200
echo "..."

# Check if response is valid JSON
if ! echo "$TUNNELS_RESPONSE" | jq empty 2>/dev/null; then
    echo -e "${RED}✗${NC} Invalid JSON response"
    exit 1
fi

TUNNEL_COUNT=$(echo "$TUNNELS_RESPONSE" | jq '.tunnels | length')
echo -e "${GREEN}✓${NC} Received valid JSON response"
echo "   Active tunnels: $TUNNEL_COUNT"

if [ "$TUNNEL_COUNT" -gt 0 ]; then
    echo ""
    echo "4. Analyzing tunnel data..."

    # Check first tunnel for "Unknown" values
    FIRST_TUNNEL=$(echo "$TUNNELS_RESPONSE" | jq '.tunnels[0]')

    SUBDOMAIN=$(echo "$FIRST_TUNNEL" | jq -r '.subdomain // "null"')
    PUBLIC_URL=$(echo "$FIRST_TUNNEL" | jq -r '.public_url // "null"')
    LOCAL_ADDR=$(echo "$FIRST_TUNNEL" | jq -r '.local_addr // "null"')
    TUNNEL_TYPE=$(echo "$FIRST_TUNNEL" | jq -r '.tunnel_type // "null"')
    STATUS=$(echo "$FIRST_TUNNEL" | jq -r '.status // "null"')

    echo "   Subdomain:   $SUBDOMAIN"
    echo "   Public URL:  $PUBLIC_URL"
    echo "   Local Addr:  $LOCAL_ADDR"
    echo "   Type:        $TUNNEL_TYPE"
    echo "   Status:      $STATUS"

    # Check for "Unknown" or empty values
    ISSUES=0

    if [ "$SUBDOMAIN" = "null" ] || [ "$SUBDOMAIN" = "" ]; then
        echo -e "${YELLOW}⚠${NC}  Subdomain is missing"
        ((ISSUES++))
    fi

    if [ "$PUBLIC_URL" = "null" ] || [ "$PUBLIC_URL" = "" ]; then
        echo -e "${YELLOW}⚠${NC}  Public URL is missing"
        ((ISSUES++))
    fi

    if [ "$LOCAL_ADDR" = "null" ] || [ "$LOCAL_ADDR" = "" ]; then
        echo -e "${YELLOW}⚠${NC}  Local address is missing"
        ((ISSUES++))
    fi

    if [ "$TUNNEL_TYPE" = "null" ] || [ "$TUNNEL_TYPE" = "" ]; then
        echo -e "${YELLOW}⚠${NC}  Tunnel type is missing"
        ((ISSUES++))
    fi

    if [ $ISSUES -eq 0 ]; then
        echo -e "${GREEN}✓${NC} All tunnel data fields are properly populated"
    else
        echo -e "${RED}✗${NC} Found $ISSUES missing data fields"
        echo ""
        echo "This indicates the 'Unknown' data issue may still exist."
        echo "Please check the client tunnel registration protocol."
        exit 1
    fi
else
    echo -e "${YELLOW}⚠${NC}  No active tunnels to verify"
    echo ""
    echo "To test with an active tunnel:"
    echo "1. Run: ./bin/grok http 3000 --subdomain test"
    echo "2. Re-run this script"
fi

echo ""
echo -e "${GREEN}=== Verification Complete ===${NC}"
