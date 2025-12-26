import { useMemo } from 'react';
import { Box } from '@mui/material';
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
} from 'reactflow';
import type { Node, Edge } from 'reactflow';
import 'reactflow/dist/style.css';
import type { WebhookRoute } from '@/lib/api';
import { Globe, Webhook, Activity } from 'lucide-react';

interface WebhookNetworkDiagramProps {
  appName: string;
  orgSubdomain: string;
  baseDomain: string;
  routes: WebhookRoute[];
}

export function WebhookNetworkDiagram({
  appName,
  orgSubdomain,
  baseDomain,
  routes,
}: WebhookNetworkDiagramProps) {
  const { nodes, edges } = useMemo(() => {
    const nodesList: Node[] = [];
    const edgesList: Edge[] = [];

    // Internet node
    nodesList.push({
      id: 'internet',
      type: 'input',
      data: {
        label: (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              padding: '12px',
            }}
          >
            <Globe style={{ height: 32, width: 32, color: '#3b82f6', marginBottom: 8 }} />
            <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>Internet</div>
            <div style={{ fontSize: '0.75rem', color: '#6b7280' }}>Webhook Sender</div>
          </div>
        ),
      },
      position: { x: 50, y: 150 },
      style: {
        background: '#eff6ff',
        border: '2px solid #3b82f6',
        borderRadius: '12px',
        padding: '8px',
        width: 180,
      },
    });

    // Webhook App node
    const webhookUrl = `${appName}-${orgSubdomain}-webhook.${baseDomain}/*`;
    nodesList.push({
      id: 'webhook-app',
      type: 'default',
      data: {
        label: (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              padding: '16px',
            }}
          >
            <Webhook style={{ height: 40, width: 40, color: '#667eea', marginBottom: 8 }} />
            <div style={{ fontSize: '0.875rem', fontWeight: 700, textAlign: 'center' }}>
              {appName}
            </div>
            <div
              style={{
                fontSize: '0.75rem',
                color: '#6b7280',
                marginTop: 4,
                textAlign: 'center',
                wordBreak: 'break-all',
              }}
            >
              {webhookUrl}
            </div>
            <div
              style={{
                fontSize: '0.75rem',
                fontWeight: 500,
                color: '#667eea',
                marginTop: 8,
              }}
            >
              Broadcast Router
            </div>
          </div>
        ),
      },
      position: { x: 350, y: 100 },
      style: {
        background: '#faf5ff',
        border: '2px solid #667eea',
        borderRadius: '12px',
        padding: '12px',
        width: 250,
      },
    });

    // Internet → Webhook App edge
    edgesList.push({
      id: 'internet-to-app',
      source: 'internet',
      target: 'webhook-app',
      label: 'POST/GET/...',
      animated: true,
      style: { stroke: '#3b82f6', strokeWidth: 2 },
      labelStyle: { fill: '#3b82f6', fontWeight: 600 },
    });

    // Tunnel nodes
    const startY = 50;
    const spacing = 180;

    routes.forEach((route, idx) => {
      const tunnelId = `tunnel-${route.tunnel_id}`;
      const isEnabled = route.is_enabled;
      const isTunnelOnline = route.tunnel?.status === 'active';

      // Determine status color based on tunnel online/offline status
      let statusColor = '#6b7280'; // gray (disabled)
      let statusBg = '#f3f4f6';
      if (isEnabled) {
        if (isTunnelOnline) {
          statusColor = '#10b981'; // green (online)
          statusBg = '#d1fae5';
        } else {
          statusColor = '#ef4444'; // red (offline)
          statusBg = '#fee2e2';
        }
      }

      nodesList.push({
        id: tunnelId,
        type: 'output',
        data: {
          label: (
            <div style={{ display: 'flex', flexDirection: 'column', padding: '12px' }}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 8,
                }}
              >
                <Activity style={{ height: 24, width: 24, color: statusColor }} />
                <div
                  style={{
                    padding: '4px 8px',
                    borderRadius: 9999,
                    fontSize: '0.75rem',
                    fontWeight: 500,
                    background: statusBg,
                    color: statusColor,
                  }}
                >
                  {isEnabled ? (isTunnelOnline ? 'Online' : 'Offline') : 'Disabled'}
                </div>
              </div>
              <div style={{ fontSize: '0.875rem', fontWeight: 600, marginBottom: 4 }}>
                {route.tunnel?.subdomain || 'Tunnel'}
              </div>
              <div style={{ fontSize: '0.75rem', color: '#6b7280', marginBottom: 8 }}>
                {route.tunnel?.local_addr || 'localhost:3000'}
              </div>
              <div style={{ display: 'flex', gap: 8, fontSize: '0.75rem' }}>
                <span style={{ color: '#6b7280' }}>Priority:</span>
                <span style={{ fontWeight: 500 }}>{route.priority}</span>
              </div>
            </div>
          ),
        },
        position: { x: 700, y: startY + idx * spacing },
        style: {
          background: isEnabled ? '#ffffff' : '#f9fafb',
          border: `2px solid ${statusColor}`,
          borderRadius: '12px',
          padding: '8px',
          width: 220,
          opacity: isEnabled ? 1 : 0.6,
        },
      });

      // Webhook App → Tunnel edge
      edgesList.push({
        id: `app-to-${route.tunnel_id}`,
        source: 'webhook-app',
        target: tunnelId,
        label: isEnabled ? `P${route.priority}` : 'Disabled',
        animated: isEnabled && isTunnelOnline,
        style: {
          stroke: statusColor,
          strokeWidth: isEnabled ? 2 : 1,
          strokeDasharray: isEnabled ? '0' : '5,5',
        },
        labelStyle: {
          fill: statusColor,
          fontWeight: 600,
          fontSize: 11,
        },
      });
    });

    return { nodes: nodesList, edges: edgesList };
  }, [appName, orgSubdomain, baseDomain, routes]);

  return (
    <Box
      sx={{
        width: '100%',
        height: 500,
        border: 1,
        borderColor: 'divider',
        borderRadius: 2,
        bgcolor: 'background.paper',
      }}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        attributionPosition="bottom-right"
        minZoom={0.5}
        maxZoom={1.5}
        defaultEdgeOptions={{
          type: 'smoothstep',
        }}
      >
        <Background color="#e5e7eb" gap={16} />
        <Controls />
        <MiniMap
          nodeColor={(node) => {
            if (node.id === 'internet') return '#3b82f6';
            if (node.id === 'webhook-app') return '#667eea';
            return '#10b981';
          }}
          maskColor="rgba(0, 0, 0, 0.1)"
        />
      </ReactFlow>
    </Box>
  );
}
