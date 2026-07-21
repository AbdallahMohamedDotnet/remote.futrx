export type AssistantPart =
  | { kind: "text"; text: string }
  | {
      kind: "tool";
      id: string;
      name: string;
      input: Record<string, unknown>;
      output?: string;
      isError?: boolean;
      status: "running" | "done";
    }
  | { kind: "thinking"; text: string };

export type AssistantBlock = {
  type: "assistant";
  parts: AssistantPart[];
  t: number;
  isComplete: boolean;
};

export type Block =
  | { type: "user"; text: string; t: number }
  | AssistantBlock
  | { type: "error"; message: string; t: number };
