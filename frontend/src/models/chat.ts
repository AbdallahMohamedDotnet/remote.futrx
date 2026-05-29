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
  projectId?: string;
}

export type ChatMode = "chat" | "plan" | "code" | "review" | "debug" | "full-auto";

export type ChatEvent =
  | { t: number; type: "user"; text: string }
  | { t: number; type: "assistant_text"; text: string; messageId?: string }
  | { t: number; type: "thinking"; text: string }
  | { t: number; type: "tool_use_start"; id: string; name: string; input: Record<string, unknown> }
  | { t: number; type: "tool_use_end"; id: string; output?: string; isError?: boolean }
  | { t: number; type: "permission_request"; id: string; toolName: string; input: Record<string, unknown> }
  | { t: number; type: "system"; subtype: string; data?: Record<string, unknown> }
  | { t: number; type: "session"; claudeSessionId: string }
  | {
      t: number;
      type: "complete";
      usage?: {
        input_tokens?: number;
        output_tokens?: number;
        cache_read_input_tokens?: number;
        cache_creation_input_tokens?: number;
      };
    }
  | { t: number; type: "error"; message: string }
  | { t: number; type: "sync"; running?: boolean };

export type ClientToServer =
  | { type: "prompt"; text: string }
  | { type: "cancel" }
  | { type: "permission"; id: string; approved: boolean };

export type ChatStatus = "loading" | "ready" | "streaming" | "error";

export interface QueuedPrompt {
  id: string;
  text: string;
}

export interface CreateChatInput {
  tmuxSession?: string;
  cwd?: string;
  title?: string;
  model?: string;
  mode?: ChatMode;
  projectId?: string;
}

export interface UpdateChatInput {
  title?: string;
  cwd?: string;
  model?: string;
  mode?: ChatMode;
}
