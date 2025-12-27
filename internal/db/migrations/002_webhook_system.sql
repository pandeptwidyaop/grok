-- Migration: 002_webhook_system
-- Description: Add webhook apps, routes, and events tables for webhook broadcast functionality
-- Created: 2024-12-26

-- ============================================================================
-- Webhook Apps Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_apps (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint: one app name per organization
    CONSTRAINT uq_webhook_apps_org_name UNIQUE (organization_id, name)
);

-- Indexes for webhook_apps
CREATE INDEX idx_webhook_apps_org ON webhook_apps(organization_id);
CREATE INDEX idx_webhook_apps_user ON webhook_apps(user_id);
CREATE INDEX idx_webhook_apps_active ON webhook_apps(is_active) WHERE is_active = TRUE;

-- ============================================================================
-- Webhook Routes Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_routes (
    id UUID PRIMARY KEY,
    webhook_app_id UUID NOT NULL REFERENCES webhook_apps(id) ON DELETE CASCADE,
    tunnel_id UUID NOT NULL REFERENCES tunnels(id) ON DELETE CASCADE,
    is_enabled BOOLEAN DEFAULT TRUE NOT NULL,
    priority INTEGER DEFAULT 100 NOT NULL,
    health_status VARCHAR(50) DEFAULT 'unknown' NOT NULL,
    failure_count INTEGER DEFAULT 0 NOT NULL,
    last_health_check TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint: one route per app-tunnel pair
    CONSTRAINT uq_webhook_routes_app_tunnel UNIQUE (webhook_app_id, tunnel_id)
);

-- Indexes for webhook_routes
CREATE INDEX idx_webhook_routes_app ON webhook_routes(webhook_app_id);
CREATE INDEX idx_webhook_routes_tunnel ON webhook_routes(tunnel_id);
CREATE INDEX idx_webhook_routes_enabled ON webhook_routes(is_enabled) WHERE is_enabled = TRUE;
CREATE INDEX idx_webhook_routes_health ON webhook_routes(health_status);

-- ============================================================================
-- Webhook Events Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY,
    webhook_app_id UUID NOT NULL REFERENCES webhook_apps(id) ON DELETE CASCADE,
    request_path VARCHAR(1024) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INTEGER,
    duration_ms BIGINT,
    bytes_in BIGINT DEFAULT 0 NOT NULL,
    bytes_out BIGINT DEFAULT 0 NOT NULL,
    client_ip VARCHAR(45),
    routing_status VARCHAR(50),
    tunnel_count INTEGER DEFAULT 0 NOT NULL,
    success_count INTEGER DEFAULT 0 NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for webhook_events
CREATE INDEX idx_webhook_events_app ON webhook_events(webhook_app_id);
CREATE INDEX idx_webhook_events_created ON webhook_events(created_at DESC);
CREATE INDEX idx_webhook_events_status ON webhook_events(status_code);
CREATE INDEX idx_webhook_events_routing ON webhook_events(routing_status);

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE webhook_apps IS 'Webhook applications that can receive broadcast events via multiple tunnels';
COMMENT ON TABLE webhook_routes IS 'Routes mapping webhook apps to specific tunnels with priority and health tracking';
COMMENT ON TABLE webhook_events IS 'Log of all webhook requests processed through the system';

COMMENT ON COLUMN webhook_apps.name IS 'Unique app name within organization (e.g., payment-app)';
COMMENT ON COLUMN webhook_apps.organization_id IS 'Organization that owns this webhook app';
COMMENT ON COLUMN webhook_apps.user_id IS 'User who created the webhook app';

COMMENT ON COLUMN webhook_routes.priority IS 'Route priority for response selection (lower = higher priority)';
COMMENT ON COLUMN webhook_routes.health_status IS 'Health status: healthy, unhealthy, unknown';
COMMENT ON COLUMN webhook_routes.failure_count IS 'Consecutive failure count for health monitoring';

COMMENT ON COLUMN webhook_events.routing_status IS 'Routing result: success, partial, failed';
COMMENT ON COLUMN webhook_events.tunnel_count IS 'Number of tunnels that received the broadcast request';
COMMENT ON COLUMN webhook_events.success_count IS 'Number of tunnels that responded successfully';
