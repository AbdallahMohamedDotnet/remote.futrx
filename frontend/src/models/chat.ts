// Provider identifiers come from the backend module catalog. Built-in string
// literals remain valid, but future modules do not require a frontend type edit.
export type ChatProvider = string;
export type ChatMode = string;
export type ReasoningEffort = string;
export type ServiceTier = string;

export interface ChatMeta {
  id: string;
  title: string;
  provider?: ChatProvider;
  sessions?: Record<string, string>;
  claudeSessionId?: string;
  codexSessionId?: string;
  kimiSessionId?: string;
  antigravitySessionId?: string;
  tmuxSession?: string;
  cwd?: string;
  createdAt: number;
  lastMessageAt: number;
  lastReadAt?: number;
  running?: boolean;
  model?: string;
  mode?: ChatMode;
  reasoningEffort?: ReasoningEffort;
  serviceTier?: ServiceTier;
  projectId?: string;
  selectedSkills?: SelectedSkill[];
}

export interface SelectedSkill {
  name: string;
  command?: string;
  provider?: ChatProvider;
  source?: string;
}

type ChatEventBase = { seq?: number; t: number };

export type InteractionAnswers = Record<string, string[]>;

export interface QuestionAnswerSubmission {
  text: string;
  preview: string;
  answers: InteractionAnswers;
  sensitive?: boolean;
}

export interface QuestionAnswerRequest extends QuestionAnswerSubmission {
  interactionId?: string;
}

export interface PromptExecutionPreferences {
  provider: ChatProvider;
  mode: ChatMode;
}

export type AnswerQuestionHandler = (answer: QuestionAnswerRequest) => boolean;
export type InteractionActivityHandler = (interactionId: string) => boolean;

export type ChatEvent = ChatEventBase & (
  | { type: "user"; text: string }
  | { type: "assistant_text"; text: string; messageId?: string }
  | { type: "thinking"; text: string }
  | { type: "tool_use_start"; id: string; name: string; input: Record<string, unknown> }
  | { type: "tool_use_end"; id: string; output?: string; isError?: boolean }
  | { type: "interaction_request"; id: string; toolName: string; input: Record<string, unknown> }
  | { type: "interaction_resolved"; id: string; output?: string; isError?: boolean }
  | { type: "permission_request"; id: string; toolName: string; input: Record<string, unknown> }
  | { type: "system"; subtype: string; data?: Record<string, unknown> }
  | { type: "session"; provider?: ChatProvider; sessionId?: string; claudeSessionId?: string; codexSessionId?: string; kimiSessionId?: string; antigravitySessionId?: string }
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
  | ({ type: "prompt"; text: string; clientId?: string } & PromptExecutionPreferences)
  | { type: "cancel" }
  | { type: "interaction_response"; id: string; answers: InteractionAnswers }
  | { type: "interaction_activity"; id: string }
  | { type: "permission"; id: string; approved: boolean };

export type ChatStatus = "loading" | "ready" | "streaming" | "error";

export interface QueuedPrompt {
  id: string;
  text: string;
  preferences: PromptExecutionPreferences;
}

// Server verdict on a prompt sent with a clientId. Retryable busy rejections
// remain queued; semantic rejections return to the draft for review.
export interface PromptOutcome {
  clientId: string;
  accepted: boolean;
  retryable: boolean;
  reason: string;
}

export interface CreateChatInput {
  tmuxSession?: string;
  cwd?: string;
  title?: string;
  provider?: ChatProvider;
  model?: string;
  mode?: ChatMode;
  reasoningEffort?: ReasoningEffort;
  serviceTier?: ServiceTier;
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
  serviceTier?: ServiceTier;
  selectedSkills?: SelectedSkill[];
}
