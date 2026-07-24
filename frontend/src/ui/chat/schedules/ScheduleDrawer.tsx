import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { chatApi } from "../../../api/chatApi";
import type { ScheduledTask } from "../../../models/schedule";
import {
  AlertCircle,
  CalendarClock,
  Loader,
  Pause,
  Play,
  RotateCcw,
  Trash,
  X,
} from "../../primitives/icons";
import {
  canResumeScheduledTask,
  formatTimestamp,
  scheduleDefinition,
  scheduleRunCount,
  sortScheduledTasks,
} from "./scheduledTaskView";

type TaskAction = "toggle" | "run" | "delete";

export function ScheduleDrawer({
  chatId,
  open,
  onClose,
}: {
  chatId: string;
  open: boolean;
  onClose: () => void;
}) {
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<{ id: string; action: TaskAction } | null>(null);
  const requestSequence = useRef(0);
  const sortedTasks = useMemo(() => sortScheduledTasks(tasks), [tasks]);
  const enabledCount = tasks.filter((task) => task.enabled).length;

  useEffect(() => {
    requestSequence.current += 1;
    setTasks([]);
    setError(null);
    setNotice(null);
    setBusy(null);
  }, [chatId]);

  useEffect(() => {
    if (!open) {
      requestSequence.current += 1;
      setNotice(null);
      return;
    }
    void load();
    return () => {
      requestSequence.current += 1;
    };
  }, [chatId, open]);

  async function load() {
    const sequence = ++requestSequence.current;
    setLoading(true);
    setError(null);
    try {
      const response = await chatApi.fetchSchedules(chatId);
      if (sequence !== requestSequence.current) return;
      setTasks(response);
    } catch (err) {
      if (sequence !== requestSequence.current) return;
      setError(errorMessage(err));
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  }

  async function toggle(task: ScheduledTask) {
    await perform(task, "toggle", async () => {
      await chatApi.updateSchedule(task.id, { enabled: !task.enabled });
    });
  }

  async function runNow(task: ScheduledTask) {
    await perform(task, "run", async () => {
      await chatApi.runSchedule(task.id);
      setNotice(`Run requested for “${task.name}”.`);
    });
  }

  async function remove(task: ScheduledTask) {
    if (!confirm(`Delete scheduled task "${task.name}"? Its run history will also be removed.`)) return;
    await perform(task, "delete", async () => {
      await chatApi.deleteSchedule(task.id);
      setNotice(`Deleted “${task.name}”.`);
    });
  }

  async function perform(
    task: ScheduledTask,
    action: TaskAction,
    mutation: () => Promise<void>
  ) {
    if (busy) return;
    setBusy({ id: task.id, action });
    setError(null);
    setNotice(null);
    try {
      await mutation();
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  const subtitle = loading && tasks.length === 0
    ? "Loading…"
    : tasks.length === 0
      ? "No tasks"
      : `${enabledCount} active · ${tasks.length} total`;

  return (
    <aside
      class={`relative z-20 h-full flex-none overflow-hidden bg-[#101318] border-l border-white/10 shadow-2xl
              transition-[width,opacity] duration-200 ease-out ${open ? "opacity-100" : "opacity-0 border-l-0 pointer-events-none"}`}
      style={{ width: open ? "min(520px, calc(100vw - 64px))" : "0px" }}
      aria-hidden={!open}
      aria-label="Scheduled tasks"
    >
      <div
        class={`h-full min-h-0 w-full flex flex-col transition-transform duration-200 ease-out
                ${open ? "translate-x-0" : "translate-x-full"}`}
      >
        <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
          <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
            <CalendarClock class="w-4 h-4 text-accent-blue" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
              Scheduled tasks
            </h2>
            <div class="truncate text-[12px] text-ink-300">{subtitle}</div>
          </div>
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading || !!busy}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:opacity-50"
            title="Refresh scheduled tasks"
            aria-label="Refresh scheduled tasks"
          >
            {loading ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
          </button>
          <button
            type="button"
            onClick={onClose}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
            title="Close scheduled tasks"
            aria-label="Close scheduled tasks"
          >
            <X class="w-4 h-4" />
          </button>
        </header>

        <div class="flex-1 min-h-0 overflow-y-auto touch-scroll px-3 md:px-4 py-3">
          {error && (
            <div class="mb-3 flex items-start gap-2.5 rounded-md border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
              <AlertCircle class="mt-0.5 h-4 w-4 flex-none text-accent-red" />
              <div class="min-w-0 flex-1 break-words text-accent-red">{error}</div>
              <button
                type="button"
                onClick={() => void load()}
                class="flex-none text-[12px] text-ink-200 underline decoration-white/30 underline-offset-2 hover:text-white"
              >
                Retry
              </button>
            </div>
          )}
          {notice && (
            <div class="mb-3 rounded-md border border-accent-green/25 bg-accent-green/[0.08] px-3 py-2 text-[12.5px] text-accent-green">
              {notice}
            </div>
          )}

          {loading && tasks.length === 0 ? (
            <div class="h-36 grid place-items-center text-[13px] text-ink-300">
              <div class="flex items-center gap-2">
                <Loader class="w-4 h-4 animate-spin" />
                Loading scheduled tasks…
              </div>
            </div>
          ) : tasks.length === 0 && !error ? (
            <EmptyScheduleState />
          ) : (
            <div class="space-y-2.5">
              {sortedTasks.map((task) => (
                <ScheduledTaskCard
                  key={task.id}
                  task={task}
                  busyAction={busy?.id === task.id ? busy.action : null}
                  actionsDisabled={!!busy}
                  onToggle={() => void toggle(task)}
                  onRun={() => void runNow(task)}
                  onDelete={() => void remove(task)}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}

function EmptyScheduleState() {
  return (
    <div class="mt-8 rounded-lg border border-dashed border-white/15 bg-white/[0.025] px-5 py-7 text-center">
      <div class="mx-auto mb-3 h-10 w-10 rounded-lg border border-white/10 bg-white/[0.05] grid place-items-center">
        <CalendarClock class="w-5 h-5 text-accent-blue" />
      </div>
      <h3 class="text-[14px] font-medium text-ink-100">No scheduled tasks</h3>
      <p class="mx-auto mt-1.5 max-w-sm text-[12.5px] leading-5 text-ink-400">
        Ask the agent to schedule work in this chat. It can create a one-time reminder or a recurring cron task.
      </p>
      <div class="mx-auto mt-4 max-w-sm rounded-md border border-white/[0.08] bg-black/20 px-3 py-2 text-left text-[12px] leading-5 text-ink-300">
        “Watch the deploy every 5 minutes and stop when it is healthy.”
      </div>
    </div>
  );
}

function ScheduledTaskCard({
  task,
  busyAction,
  actionsDisabled,
  onToggle,
  onRun,
  onDelete,
}: {
  task: ScheduledTask;
  busyAction: TaskAction | null;
  actionsDisabled: boolean;
  onToggle: () => void;
  onRun: () => void;
  onDelete: () => void;
}) {
  const resumeAllowed = canResumeScheduledTask(task);
  const toggleDisabled = actionsDisabled || (!task.enabled && !resumeAllowed);

  return (
    <article class={`rounded-lg border px-3 py-3 ${task.enabled ? "border-white/10 bg-white/[0.035]" : "border-white/[0.07] bg-white/[0.018]"}`}>
      <div class="flex items-start gap-2">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 min-w-0">
            <h3 class="truncate text-[14px] font-medium text-ink-50">{task.name}</h3>
            <TaskStatus task={task} />
          </div>
          <div class="mt-1 truncate font-mono text-[11.5px] text-accent-blue/90" title={scheduleDefinition(task)}>
            {scheduleDefinition(task)}
          </div>
        </div>
        <span class="flex-none text-[11px] text-ink-400">{scheduleRunCount(task)}</span>
      </div>

      <p class="mt-2 line-clamp-3 whitespace-pre-wrap break-words text-[12.5px] leading-[1.55] text-ink-300" title={task.prompt}>
        {task.prompt}
      </p>

      <dl class="mt-3 grid grid-cols-2 gap-x-3 gap-y-2 rounded-md border border-white/[0.06] bg-black/15 px-2.5 py-2">
        <TaskDetail
          label="Next"
          value={task.enabled
            ? formatTimestamp(task.nextRunAt)
            : humanize(task.status || "paused")}
        />
        <TaskDetail label="Last run" value={formatTimestamp(task.lastRunAt)} />
        <TaskDetail label="Last result" value={task.lastRunStatus ? humanize(task.lastRunStatus) : "None"} tone={lastRunTone(task.lastRunStatus)} />
        <TaskDetail label="Owner" value={task.ownerEmail || "Unknown"} />
      </dl>

      {task.lastError && (
        <div class="mt-2 rounded-md border border-accent-red/20 bg-accent-red/[0.06] px-2.5 py-2 text-[11.5px] leading-4 text-accent-red break-words">
          {task.lastError}
        </div>
      )}

      <div class="mt-3 flex items-center gap-2">
        <button
          type="button"
          onClick={onToggle}
          disabled={toggleDisabled}
          class="h-8 inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-white/[0.04] px-2.5 text-[12px] text-ink-200 hover:bg-white/[0.08] disabled:opacity-45"
          title={
            task.enabled
              ? "Pause schedule"
              : resumeAllowed
                ? "Resume schedule"
                : `${humanize(task.status || "terminal")} tasks cannot be resumed`
          }
        >
          {busyAction === "toggle"
            ? <Loader class="w-3.5 h-3.5 animate-spin" />
            : task.enabled
              ? <Pause class="w-3.5 h-3.5" />
              : <Play class="w-3.5 h-3.5" />}
          {task.enabled ? "Pause" : "Resume"}
        </button>
        <button
          type="button"
          onClick={onRun}
          disabled={actionsDisabled}
          class="h-8 inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-white/[0.04] px-2.5 text-[12px] text-ink-200 hover:bg-white/[0.08] disabled:opacity-45"
          title="Run this task now"
        >
          {busyAction === "run" ? <Loader class="w-3.5 h-3.5 animate-spin" /> : <Play class="w-3.5 h-3.5" />}
          Run now
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={actionsDisabled}
          class="ml-auto h-8 w-8 rounded-md border border-white/10 bg-white/[0.03] text-ink-400 grid place-items-center hover:bg-accent-red/[0.08] hover:text-accent-red disabled:opacity-45"
          title="Delete scheduled task"
          aria-label={`Delete ${task.name}`}
        >
          {busyAction === "delete" ? <Loader class="w-3.5 h-3.5 animate-spin" /> : <Trash class="w-3.5 h-3.5" />}
        </button>
      </div>
    </article>
  );
}

function TaskStatus({ task }: { task: ScheduledTask }) {
  const label = !task.enabled && task.status === "scheduled" ? "paused" : task.status || (task.enabled ? "scheduled" : "paused");
  const classes = task.enabled
    ? "border-accent-green/25 bg-accent-green/[0.08] text-accent-green"
    : "border-white/10 bg-white/[0.04] text-ink-400";
  return (
    <span class={`flex-none rounded-full border px-1.5 py-0.5 text-[9.5px] font-medium uppercase tracking-wide ${classes}`}>
      {humanize(label)}
    </span>
  );
}

function TaskDetail({
  label,
  value,
  tone = "text-ink-200",
}: {
  label: string;
  value: string;
  tone?: string;
}) {
  return (
    <div class="min-w-0">
      <dt class="text-[10px] uppercase tracking-wide text-ink-500">{label}</dt>
      <dd class={`mt-0.5 truncate text-[11.5px] ${tone}`} title={value}>{value}</dd>
    </div>
  );
}

function lastRunTone(status?: string): string {
  const normalized = (status || "").toLowerCase();
  if (["failed", "error"].includes(normalized)) return "text-accent-red";
  if (["success", "succeeded", "complete", "completed"].includes(normalized)) return "text-accent-green";
  return "text-ink-200";
}

function humanize(value: string): string {
  return value.replaceAll("_", " ");
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
