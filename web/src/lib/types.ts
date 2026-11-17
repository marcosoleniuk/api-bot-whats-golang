export type SessionStatus = "pending" | "connected" | "disconnected" | "error" | string;

export interface Envelope<T> {
  status: "success" | "error" | string;
  message: string;
  data?: T;
  code?: string;
  details?: Record<string, string>;
  timestamp?: string;
}

export interface WaSession {
  id: string;
  whatsapp_session_key: string;
  nome_pessoa: string;
  email_pessoa: string;
  phone_number?: string;
  status: SessionStatus;
  created_at: string;
  last_connected_at?: string;
}

export interface SessionsData {
  total: number;
  sessions: WaSession[];
}

export interface Health {
  status: string;
  service: string;
  version: string;
  whatsapp_version?: string;
  uptime: string;
  timestamp: string;
  checks: Record<string, string>;
}

export interface Webhook {
  id: string;
  session_id: string;
  tenant_id: string;
  url: string;
  events: string[];
  headers?: Record<string, string>;
  active: boolean;
  created_at: string;
  updated_at: string;
  last_triggered_at?: string;
}

export interface WebhookLog {
  id: string;
  webhook_id: string;
  event_type: string;
  payload: string;
  http_status?: number;
  response_body?: string;
  error_message?: string;
  execution_time_ms?: number;
  triggered_at: string;
}

export interface ChatMessage {
  id: string;
  direction: "inbound" | "outbound" | string;
  sender?: string;
  recipient?: string;
  message_type: string;
  content?: string;
  media_url?: string;
  media_download_url?: string;
  mime_type?: string;
  status: string;
  created_at: string;
  [key: string]: unknown;
}

export interface Conversation {
  messages: ChatMessage[];
  pagination: { total: number; limit: number; offset: number };
}

export interface MessageStats {
  total_messages: number;
  inbound: number;
  outbound: number;
  contact_count: number;
}

/**
 * Eventos que disparam webhooks no serviço Go ("*" = todos).
 * Os eventos session.* e message.receipt existem só no WebSocket.
 */
export const WEBHOOK_EVENTS = ["message.sent", "message.received", "message.read", "*"] as const;
