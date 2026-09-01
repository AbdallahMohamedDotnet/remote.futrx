export type AssistantMessagePart =
  | { kind: "text"; text: string }
  | {
      kind: "tool";
      id: string;
      name: string;
      input: Record<string, unknown>;
      output?: string;
      isError?: boolean;
      status: "running" | "done";
      interactive?: boolean;
      interactionRequestedAt?: number;
    }
  | { kind: "thinking"; text: string };

export type AssistantMessageBlock = {
  type: "assistant";
  parts: AssistantMessagePart[];
  t: number;
  isComplete: boolean;
};

export type ChatMessageBlock =
  | { type: "user"; text: string; t: number }
  | AssistantMessageBlock
  | { type: "error"; message: string; t: number };
