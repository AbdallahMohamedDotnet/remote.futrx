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
  mode?: ChatMode;
  // When set, claude spawns inside the project's LXC container. When empty,
  // legacy host-spawn mode (or the chat predates the projects feature).
  projectId?: string;
}

export type ChatMode = "chat" | "plan" | "code" | "review" | "debug" | "full-auto";

// Mirror of backend internal/projects/types.go ProjectStatus.
export type ProjectStatus =
  | ""              // unknown
  | "provisioning"  // container is being launched
  | "running"       // container is up
  | "stopped"       // container exists but is not running
  | "error"         // last operation failed; see errorMsg
  | "missing";      // meta exists but no container — needs reprovision

export interface ProjectMeta {
  id: string;
  name: string;
  slug: string;
  cwd: string;            // host workspace path; mount inside container at /workspace
  containerName: string;  // LXC container name (e.g. "proj-demo-app")
  status: ProjectStatus;
  errorMsg?: string;
  createdAt: number;
  updatedAt: number;
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
  | { t: number; type: "error"; message: string }
  | { t: number; type: "sync"; running?: boolean };

export type ClientToServer =
  | { type: "prompt"; text: string }
  | { type: "cancel" }
  | { type: "permission"; id: string; approved: boolean };

export type SessionView = "terminal" | "chat";
