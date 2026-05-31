import type { ProjectContainerRecords } from "../../containers/ProjectContainersContainer";
import type { ProjectMeta } from "../../models/project";
import { AlertCircle, ChevronLeft, Loader, Menu, RotateCcw, Settings } from "../ui/icons";

export function ProjectContainersPage({
  projects,
  selectedProjectId,
  records,
  refreshing,
  onRefresh,
  onBack,
  onHamburger,
}: {
  projects: ProjectMeta[];
  selectedProjectId: string | null;
  records: ProjectContainerRecords;
  refreshing: boolean;
  onRefresh: () => void;
  onBack: () => void;
  onHamburger: () => void;
}) {
  return (
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <header class="codex-header top-chrome flex-none z-20 bg-[#101318] border-b border-white/10 px-3 pb-2 flex items-center gap-2 min-h-[52px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 text-ink-100 rounded-md hover:bg-white/[0.08] grid place-items-center"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <button
          type="button"
          onClick={onBack}
          class="hidden md:inline-flex items-center gap-1.5 h-10 px-2 text-ink-200 hover:text-ink-50
                 hover:bg-white/[0.08] rounded-md text-sm"
        >
          <ChevronLeft class="w-4 h-4" /> Chats
        </button>
        <div class="flex-1 min-w-0">
          <div class="text-[11px] text-ink-300">Projects</div>
          <div class="text-[15px] font-semibold text-ink-50 truncate">Container info</div>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          disabled={refreshing}
          class="h-10 w-10 rounded-md text-ink-300 hover:text-ink-50 hover:bg-white/[0.08]
                 disabled:cursor-wait grid place-items-center"
          aria-label="Refresh container info"
          title="Refresh"
        >
          {refreshing ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
        </button>
      </header>

      <div class="flex-1 overflow-y-auto touch-scroll">
        <div class="max-w-3xl mx-auto px-4 py-5 space-y-4">
          {projects.length === 0 ? (
            <div class="rounded-lg border border-white/10 bg-[#101318] px-4 py-5 text-sm text-ink-300">
              No projects yet.
            </div>
          ) : (
            projects.map((project) => (
              <ProjectContainerCard
                key={project.id}
                project={project}
                selected={project.id === selectedProjectId}
                record={records[project.id] ?? { loading: true }}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function ProjectContainerCard({
  project,
  selected,
  record,
}: {
  project: ProjectMeta;
  selected: boolean;
  record: ProjectContainerRecords[string];
}) {
  return (
    <section
      class={`rounded-lg border bg-[#101318] overflow-hidden ${
        selected ? "border-accent-blue/45" : "border-white/10"
      }`}
    >
      <header class="px-4 py-3 flex items-start gap-3 border-b border-white/[0.06]">
        <div class="mt-0.5 w-9 h-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
          <Settings class="w-4 h-4 text-ink-200" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-[14.5px] font-semibold text-ink-50 truncate">{project.name}</span>
            <span class="inline-flex items-center h-5 px-1.5 rounded bg-white/[0.06] text-[11px] text-ink-300">
              {project.status || "unknown"}
            </span>
          </div>
          <div class="text-[12.5px] text-ink-300 mt-0.5 leading-snug font-mono truncate">
            {project.containerName || project.slug}
          </div>
        </div>
        {record.loading && <Loader class="w-4 h-4 mt-2 text-ink-300 animate-spin" />}
      </header>

      <div class="p-4 space-y-4">
        <div class="grid gap-2 sm:grid-cols-2">
          <MetaRow label="Project" value={project.slug} />
          <MetaRow label="Container" value={project.containerName || "-"} />
          <MetaRow label="Status" value={project.status || "unknown"} />
          <MetaRow label="Workspace" value={project.cwd || "-"} />
        </div>

        {record.error ? (
          <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
            <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
            <div class="min-w-0">
              <div class="font-medium text-accent-red">Container endpoint unavailable</div>
              <div class="text-ink-200 mt-0.5 break-words">
                GET /api/projects/{project.id}/container returned {record.error}.
              </div>
            </div>
          </div>
        ) : record.loading ? (
          <div class="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-6 text-center text-sm text-ink-300">
            Loading container data
          </div>
        ) : (
          <pre class="max-h-[420px] overflow-auto scrollbar-thin rounded-lg border border-white/10 bg-[#090b0f] p-3 text-[12px] leading-relaxed text-ink-100">
            {JSON.stringify(record.data ?? {}, null, 2)}
          </pre>
        )}
      </div>
    </section>
  );
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2 min-w-0">
      <div class="text-[11px] text-ink-400">{label}</div>
      <div class="mt-0.5 text-[12.5px] text-ink-100 font-mono truncate" title={value}>
        {value}
      </div>
    </div>
  );
}
