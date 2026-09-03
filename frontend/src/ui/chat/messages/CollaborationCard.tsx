import type { AssistantMessagePart } from "../../../models/chatMessage";

type CollaborationPart = Extract<AssistantMessagePart, { kind: "collaboration" }>;

export function CollaborationCard({ part }: { part: CollaborationPart }) {
  const states = isObject(part.data.agentsStates) ? part.data.agentsStates : {};
  const receivers = Array.isArray(part.data.receiverThreadIds)
    ? part.data.receiverThreadIds.filter((item): item is string => typeof item === "string")
    : [];
  const label = part.name || "Subagent orchestration";
  const status = part.status || "inProgress";
  return (
    <section class="my-2 overflow-hidden rounded-lg border border-line bg-surface">
      <header class="flex items-center justify-between gap-3 border-b border-line bg-tint px-3 py-2">
        <div>
          <div class="text-[12px] font-semibold text-ink-100">{label}</div>
          {receivers.length > 0 && <div class="mt-0.5 font-mono text-[10px] text-ink-400">{receivers.join(", ")}</div>}
        </div>
        <span class="rounded-full border border-line-strong px-2 py-0.5 text-[10px] text-ink-300">{statusLabel(status)}</span>
      </header>
      <div class="space-y-2 p-3">
        {typeof part.data.prompt === "string" && (
          <p class="text-[12px] leading-relaxed text-ink-300">{part.data.prompt}</p>
        )}
        {Object.entries(states).length === 0 ? (
          <p class="text-[11px] text-ink-400">{emptyStateMessage(status)}</p>
        ) : Object.entries(states).map(([threadId, value]) => {
          const state = isObject(value) ? value : {};
          return (
            <div key={threadId} class="rounded-control border border-line bg-canvas px-2.5 py-2">
              <div class="flex items-center justify-between gap-2">
                <span class="truncate font-mono text-[10px] text-ink-300">{threadId}</span>
                <span class="text-[10px] text-ink-400">{typeof state.status === "string" ? state.status : "unknown"}</span>
              </div>
              {typeof state.message === "string" && state.message && (
                <div class="codex-prose mt-2 whitespace-pre-wrap text-[12px] leading-relaxed text-ink-200">{state.message}</div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function statusLabel(status: string): string {
  return status === "turnEnded" ? "turn ended" : status;
}

function emptyStateMessage(status: string): string {
  if (status === "inProgress") return "Waiting for subagent status…";
  if (status === "turnEnded") {
    return "The turn ended before the provider reported a final subagent status.";
  }
  return "No final subagent status was reported.";
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
