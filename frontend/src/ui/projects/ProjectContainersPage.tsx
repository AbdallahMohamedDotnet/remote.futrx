import { useState } from "preact/hooks";
import type { JSX } from "preact";
import type {
  AccessRecord,
  ProjectContainerRecord,
  SecretsRecord,
} from "../../state/projects/projectContainerRecords";
import { Empty } from "./project-containers/ProjectContainerPrimitives";
import { ProjectActions } from "./project-containers/ProjectActions";
import {
  ContainerStateBadge,
  ProjectInfoSection,
} from "./project-containers/ProjectInfoSection";
import { ProjectSecretsSection } from "./project-containers/ProjectSecretsSection";
import { ProjectSharingSection } from "./project-containers/ProjectSharingSection";
import {
  formatRelativeTime as fmtRelative,
  truncate,
} from "./project-containers/projectContainerFormat";
import type { ProjectContainerInfo, ProjectMeta } from "../../models/project";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Loader,
  Menu,
  RotateCcw,
  Settings,
} from "../primitives/icons";

export function ProjectContainersPage({
  project,
  infoRecord,
  secretsRecord,
  accessRecord,
  refreshing,
  onRefresh,
  onBack,
  onHamburger,
  onSaveSecret,
  onDeleteSecret,
  onAddMember,
  onRemoveMember,
  onRepairNetwork,
  onStartProject,
  onStopProject,
  onRestartProject,
  onDeleteProject,
}: {
  project: ProjectMeta | null;
  infoRecord: ProjectContainerRecord;
  secretsRecord: SecretsRecord;
  accessRecord: AccessRecord;
  refreshing: boolean;
  onRefresh: () => void;
  onBack: () => void;
  onHamburger: () => void;
  onSaveSecret: (key: string, value: string) => Promise<void>;
  onDeleteSecret: (key: string) => Promise<void>;
  onAddMember: (email: string) => Promise<void>;
  onRemoveMember: (email: string) => Promise<void>;
  onRepairNetwork: () => Promise<void>;
  onStartProject: () => Promise<void>;
  onStopProject: () => Promise<void>;
  onRestartProject: () => Promise<void>;
  onDeleteProject: () => Promise<void>;
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
          <div class="text-[15px] font-semibold text-ink-50 truncate">
            {project ? `${project.name} — container` : "Container info"}
          </div>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          disabled={refreshing}
          class="h-10 w-10 rounded-md text-ink-300 hover:text-ink-50 hover:bg-white/[0.08]
                 disabled:cursor-wait grid place-items-center"
          aria-label="Refresh"
          title="Refresh"
        >
          {refreshing ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
        </button>
      </header>

      <div class="flex-1 overflow-y-auto touch-scroll">
        <div class="max-w-3xl mx-auto px-4 py-5 space-y-3">
          {!project ? (
            <Empty text="Select a project from the sidebar." />
          ) : (
            <>
              <ProjectHeader project={project} info={infoRecord.data} refreshedAt={infoRecord.refreshedAt} />
              <CollapsibleSection title="Info" defaultOpen={true} subtitle={infoSubtitle(infoRecord)}>
                <ProjectInfoSection
                  project={project}
                  record={infoRecord}
                  onRepairNetwork={onRepairNetwork}
                />
              </CollapsibleSection>
              <CollapsibleSection title="Settings" defaultOpen={false} subtitle={project.status || "unknown"}>
                <ProjectActions
                  project={project}
                  onStart={onStartProject}
                  onStop={onStopProject}
                  onRestart={onRestartProject}
                  onDelete={onDeleteProject}
                />
              </CollapsibleSection>
              <CollapsibleSection
                title="Secrets"
                defaultOpen={false}
                subtitle={secretsSubtitle(secretsRecord)}
              >
                <ProjectSecretsSection
                  record={secretsRecord}
                  onSave={onSaveSecret}
                  onDelete={onDeleteSecret}
                />
              </CollapsibleSection>
              <CollapsibleSection
                title="Sharing"
                defaultOpen={false}
                subtitle={accessSubtitle(accessRecord)}
              >
                <ProjectSharingSection
                  record={accessRecord}
                  onAdd={onAddMember}
                  onRemove={onRemoveMember}
                />
              </CollapsibleSection>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function infoSubtitle(r: ProjectContainerRecord): string {
  if (r.loading && !r.data) return "loading…";
  if (r.error) return "error";
  if (r.data) return `${r.data.state.toLowerCase()}${r.data.image ? ` · ${truncate(r.data.image, 40)}` : ""}`;
  return "";
}

function secretsSubtitle(r: SecretsRecord): string {
  if (r.loading && !r.data) return "loading…";
  if (r.error) return "error";
  const n = r.data?.length ?? 0;
  return `${n} secret${n === 1 ? "" : "s"}`;
}

function accessSubtitle(r: AccessRecord): string {
  if (r.loading && !r.data) return "loading…";
  if (r.error) return "error";
  const n = r.data?.length ?? 0;
  return `${n} member${n === 1 ? "" : "s"}`;
}

function CollapsibleSection({
  title,
  subtitle,
  defaultOpen,
  children,
}: {
  title: string;
  subtitle?: string;
  defaultOpen?: boolean;
  children: JSX.Element | JSX.Element[];
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <section class="rounded-lg border border-white/10 bg-[#101318] overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        class="w-full px-4 py-3 flex items-center gap-2 hover:bg-white/[0.03] text-left"
      >
        {open ? (
          <ChevronDown class="w-4 h-4 text-ink-300 flex-none" />
        ) : (
          <ChevronRight class="w-4 h-4 text-ink-300 flex-none" />
        )}
        <span class="text-[13px] font-semibold text-ink-50">{title}</span>
        {subtitle && (
          <span class="text-[11.5px] text-ink-400 truncate min-w-0">{subtitle}</span>
        )}
      </button>
      {open && <div class="border-t border-white/[0.06] p-3 space-y-3">{children}</div>}
    </section>
  );
}

function ProjectHeader({
  project,
  info,
  refreshedAt,
}: {
  project: ProjectMeta;
  info?: ProjectContainerInfo;
  refreshedAt?: number;
}) {
  return (
    <section class="rounded-lg border border-white/10 bg-[#101318] px-4 py-3 flex items-start gap-3">
      <div class="mt-0.5 w-9 h-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
        <Settings class="w-4 h-4 text-ink-200" />
      </div>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 min-w-0">
          <span class="text-[14.5px] font-semibold text-ink-50 truncate">{project.name}</span>
          {info && <ContainerStateBadge state={info.state ?? "UNKNOWN"} />}
        </div>
        <div class="text-[12.5px] text-ink-300 mt-0.5 leading-snug font-mono truncate">
          {project.containerName || project.slug}
        </div>
      </div>
      {refreshedAt && (
        <div class="text-[11px] text-ink-400 mt-1.5">refreshed {fmtRelative(refreshedAt)}</div>
      )}
    </section>
  );
}
