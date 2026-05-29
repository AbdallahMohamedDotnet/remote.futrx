export interface AskInput {
  questions?: Array<{
    question: string;
    header?: string;
    multiSelect?: boolean;
    options: Array<{ label: string; description?: string }>;
  }>;
}

export interface ToolCallProps {
  toolUseId?: string;
  chatId?: string;
  name: string;
  input: Record<string, unknown> | undefined;
  output?: string;
  isError?: boolean;
  status: "running" | "done";
  onAnswerQuestion?: (text: string) => void;
}
