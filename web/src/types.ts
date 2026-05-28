// Shared types between client and (implicitly) the Go server.

export interface Session {
  name: string;
  created: number;
  attached: boolean;
  windows: number;
  cwd: string;
}

export interface ChatMeta {
  id: string;
  title: string;
  claudeSessionId?: string;
  tmuxSession?: string;
  cwd?: string;
  createdAt: number;
  lastMessageAt: number;
  model?: string;
}

// Events appended to events.jsonl AND streamed live over /ws/chat/{id}.
// These are our internal shape — not 1:1 with claude's stream-json. The Go
// server normalizes claude events into this shape.
export type ChatEvent =
  | { t: number; type: "user"; text: string }
  | { t: number; type: "assistant_text"; text: string; messageId?: string }
  | { t: number; type: "thinking"; text: string }
  | { t: number; type: "tool_use_start"; id: string; name: string; input: Record<string, unknown> }
  | { t: number; type: "tool_use_end"; id: string; output?: string; isError?: boolean }
  | { t: number; type: "permission_request"; id: string; toolName: string; input: Record<string, unknown> }
  | { t: number; type: "system"; subtype: string; data?: Record<string, unknown> }
  | { t: number; type: "session"; claudeSessionId: string }
  | { t: number; type: "complete"; usage?: { input_tokens?: number; output_tokens?: number; cache_read_input_tokens?: number } }
  | { t: number; type: "error"; message: string };

export type ClientToServer =
  | { type: "prompt"; text: string }
  | { type: "cancel" }
  | { type: "permission"; id: string; approved: boolean };

export type SessionView = "terminal" | "chat";
