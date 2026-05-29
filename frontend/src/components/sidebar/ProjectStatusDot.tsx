import type { ProjectStatus } from "../../models/project";

export function ProjectStatusDot({ status }: { status: ProjectStatus }) {
  if (status === "provisioning") {
    return (
      <span
        class="flex-none w-2 h-2 rounded-full bg-accent-yellow animate-pulse"
        title="Provisioning"
      />
    );
  }

  const cls: Record<string, string> = {
    running: "bg-accent-green",
    stopped: "bg-ink-400",
    error: "bg-accent-red",
    missing: "bg-accent-red/60 ring-1 ring-accent-red",
    "": "bg-ink-400",
  };
  const label: Record<string, string> = {
    running: "Running",
    stopped: "Stopped",
    error: "Error",
    missing: "Missing - needs reprovision",
    "": "Unknown",
  };

  return (
    <span
      class={`flex-none w-2 h-2 rounded-full ${cls[status] ?? "bg-ink-400"}`}
      title={label[status] ?? "Unknown"}
    />
  );
}
