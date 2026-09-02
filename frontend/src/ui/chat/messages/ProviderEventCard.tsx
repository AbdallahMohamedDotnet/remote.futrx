import type { AssistantMessagePart } from "../../../models/chatMessage";

type ProviderPart = Extract<AssistantMessagePart, { kind: "provider-event" }>;

export function ProviderEventCard({ part }: { part: ProviderPart }) {
  return (
    <details class="my-2 rounded-control border border-dashed border-line-strong bg-tint px-3 py-2">
      <summary class="cursor-pointer font-mono text-[10px] text-ink-400">
        Codex · {part.name}{part.status ? ` · ${part.status}` : ""}
      </summary>
      {part.data !== undefined && (
        <pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all font-mono text-[10px] leading-relaxed text-ink-400">
          {formatData(part.data)}
        </pre>
      )}
    </details>
  );
}

function formatData(data: unknown): string {
  if (typeof data === "string") return data;
  try {
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
}
