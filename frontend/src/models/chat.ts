export interface ChatMeta {
  id: string;
  title: string;
  provider?: ChatProvider;
  claudeSessionId?: string;
  codexSessionId?: string;
  kimiSessionId?: string;
  tmuxSession?: string;
  cwd?: string;
  createdAt: number;
  lastMessageAt: number;
  lastReadAt?: number;
  running?: boolean;
  model?: string;
  mode?: ChatMode;
  reasoningEffort?: ReasoningEffort;
  projectId?: string;
  selectedSkills?: SelectedSkill[];
}

export type ChatProvider = "claude" | "codex" | "kimi";
export type ChatMode = "chat" | "plan" | "code" | "review" | "debug" | "full-auto";
export type ReasoningEffort = "" | "low" | "medium" | "high" | "xhigh";

export interface SelectedSkill {
  name: string;
  command?: string;
  provider?: ChatProvider;
  source?: string;
}

type ChatEventBase = { seq?: number; t: number };

export type ChatEvent = ChatEventBase & (
  | { type: "user"; text: string }
  | { type: "assistant_text"; text: string; messageId?: string }
  | { type: "thinking"; text: string }
  | { type: "tool_use_start"; id: string; name: string; input: Record<string, unknown> }
  | { type: "tool_use_end"; id: string; output?: string; isError?: boolean }
  | { type: "permission_request"; id: string; toolName: string; input: Record<string, unknown> }
  | { type: "system"; subtype: string; data?: Record<string, unknown> }
  | { type: "session"; provider?: ChatProvider; claudeSessionId?: string; codexSessionId?: string; kimiSessionId?: string }
  | {
      type: "complete";
      usage?: {
        input_tokens?: number;
        output_tokens?: number;
        cache_read_input_tokens?: number;
        cache_creation_input_tokens?: number;
      };
    }
  | { type: "error"; message: string }
  | { type: "sync"; running?: boolean }
);

export interface ChatEventPage {
  events: ChatEvent[];
  nextBefore?: number;
  lastSeq: number;
  hasMore: boolean;
}

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
  provider?: ChatProvider;
  model?: string;
  mode?: ChatMode;
  reasoningEffort?: ReasoningEffort;
  projectId?: string;
  selectedSkills?: SelectedSkill[];
}

export interface UpdateChatInput {
  title?: string;
  cwd?: string;
  provider?: ChatProvider;
  model?: string;
  mode?: ChatMode;
  reasoningEffort?: ReasoningEffort;
  selectedSkills?: SelectedSkill[];
}
