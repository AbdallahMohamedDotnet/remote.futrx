import type { AssistantMessagePart } from "../../../models/chatMessage";

export type InteractionPart = Extract<AssistantMessagePart, { kind: "interaction" }>;

export interface InteractionFormProps {
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result?: unknown, error?: unknown) => void;
}

export interface UserQuestion {
  id?: string;
  header?: string;
  question?: string;
  options?: Array<{ label?: string; description?: string }>;
  isOther?: boolean;
  isSecret?: boolean;
}
