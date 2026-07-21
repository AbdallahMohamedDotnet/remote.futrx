import { useState } from "preact/hooks";
import type { ProjectMeta } from "../../../models/project";
import { AlertCircle } from "../../primitives/icons";

export function ProjectActions({
  project,
  onStart,
  onStop,
  onDelete,
}: {
  project: ProjectMeta;
  onStart: () => Promise<void>;
  onStop: () => Promise<void>;
  onDelete: () => Promise<void>;
}) {
  const [busy, setBusy] = useState<"start" | "stop" | "delete" | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const canStart = project.status === "stopped" || project.status === "missing" || project.status === "error";
  const canStop = project.status === "running";

  async function run(action: "start" | "stop" | "delete", operation: () => Promise<void>) {
    if (action === "delete" && !confirm(`Delete project "${project.name}"? This destroys the container and removes project settings.`)) return;
    setBusy(action);
    setErr(null);
    try {
      await operation();
    } catch (error) {
      setErr((error as Error).message);
    } finally {
      setBusy(null);
    }
  }

  return (
    <div class="space-y-3">
      {err && (
        <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
          <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
          <div class="text-accent-red break-words">{err}</div>
        </div>
      )}
      <div class="grid gap-2 sm:grid-cols-2">
        <button
          type="button"
          onClick={() => void run("start", onStart)}
          disabled={!canStart || busy !== null}
          class="h-10 rounded-md border border-white/10 bg-white/[0.04] px-3 text-[13px] font-medium text-ink-100 hover:bg-white/[0.08] disabled:opacity-45 disabled:cursor-not-allowed"
        >
          {busy === "start" ? "Starting..." : "Start project"}
        </button>
        <button
          type="button"
          onClick={() => void run("stop", onStop)}
          disabled={!canStop || busy !== null}
          class="h-10 rounded-md border border-white/10 bg-white/[0.04] px-3 text-[13px] font-medium text-ink-100 hover:bg-white/[0.08] disabled:opacity-45 disabled:cursor-not-allowed"
        >
          {busy === "stop" ? "Stopping..." : "Stop project"}
        </button>
      </div>
      <button
        type="button"
        onClick={() => void run("delete", onDelete)}
        disabled={busy !== null}
        class="h-10 w-full rounded-md border border-accent-red/30 bg-accent-red/[0.08] px-3 text-[13px] font-semibold text-accent-red hover:bg-accent-red/[0.14] disabled:opacity-45 disabled:cursor-not-allowed"
      >
        {busy === "delete" ? "Deleting..." : "Delete project"}
      </button>
    </div>
  );
}
