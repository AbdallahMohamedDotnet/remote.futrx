import { useState } from "preact/hooks";
import type { JSX } from "preact";
import type {
  AccessRecord,
  ProjectContainerRecord,
  SecretsRecord,
} from "../../state/projects/projectContainerRecords";
import type {
  AuthBundleFileStatus,
  AuthBundleStatus,
  ContainerLimits,
  DiskUsage,
  NetworkInterface,
  OSInfo,
  ProjectContainerInfo,
  ProjectMeta,
  ProjectSecret,
  ResourceInfo,
  WorkspaceInfo,
} from "../../models/project";
import {
  AlertCircle,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Loader,
  Menu,
  RotateCcw,
  Settings,
  X,
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
  onRefreshSecrets,
  onRepairNetwork,
  onStartProject,
  onStopProject,
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
  onRefreshSecrets: () => void;
  onRepairNetwork: () => Promise<void>;
  onStartProject: () => Promise<void>;
  onStopProject: () => Promise<void>;
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
                <InfoBody project={project} record={infoRecord} onRepairNetwork={onRepairNetwork} />
              </CollapsibleSection>
              <CollapsibleSection title="Settings" defaultOpen={false} subtitle={project.status || "unknown"}>
                <ProjectActions
                  project={project}
                  onStart={onStartProject}
                  onStop={onStopProject}
                  onDelete={onDeleteProject}
                />
              </CollapsibleSection>
              <CollapsibleSection
                title="Secrets"
                defaultOpen={false}
                subtitle={secretsSubtitle(secretsRecord)}
              >
                <SecretsBody
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
                <SharingBody
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

function ProjectActions({
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

  async function run(action: "start" | "stop" | "delete", fn: () => Promise<void>) {
    if (action === "delete" && !confirm(`Delete project "${project.name}"? This destroys the container and removes project settings.`)) return;
    setBusy(action);
    setErr(null);
    try {
      await fn();
    } catch (e) {
      setErr((e as Error).message);
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

function SharingBody({
  record,
  onAdd,
  onRemove,
}: {
  record: AccessRecord;
  onAdd: (email: string) => Promise<void>;
  onRemove: (email: string) => Promise<void>;
}) {
  return (
    <>
      {record.error && (
        <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
          <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
          <div class="text-accent-red break-words">{record.error}</div>
        </div>
      )}
      <AddMemberForm onAdd={onAdd} />
      <MembersList
        members={record.data ?? []}
        loading={record.loading && !record.data}
        onRemove={onRemove}
      />
      <p class="text-[11.5px] text-ink-400 leading-relaxed">
        Members can use this project — terminal, chats, secrets, uploads, browser. To add someone here they must first appear in the global Users panel (Account &rarr; Users).
      </p>
    </>
  );
}

function AddMemberForm({
  onAdd,
}: {
  onAdd: (email: string) => Promise<void>;
}) {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async (e: Event) => {
    e.preventDefault();
    const em = email.trim().toLowerCase();
    if (!em) {
      setErr("Email is required.");
      return;
    }
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(em)) {
      setErr("That doesn't look like an email.");
      return;
    }
    setErr(null);
    setSubmitting(true);
    try {
      await onAdd(em);
      setEmail("");
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={submit} class="rounded-md border border-white/10 bg-white/[0.03] p-2.5 space-y-2">
      <div class="grid gap-2 sm:grid-cols-[1fr_auto] items-center">
        <input
          type="email"
          value={email}
          onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
          placeholder="someone@example.com"
          spellcheck={false}
          autoComplete="off"
          class="h-9 px-2.5 rounded border border-white/10 bg-black/30 text-[13px] text-ink-50 placeholder-ink-400 focus:outline-none focus:border-accent-blue/50"
        />
        <button
          type="submit"
          disabled={submitting}
          class="h-9 px-3 rounded bg-accent-blue/80 hover:bg-accent-blue text-white text-[13px] font-medium disabled:opacity-50"
        >
          {submitting ? "Adding…" : "Add"}
        </button>
      </div>
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </form>
  );
}

function MembersList({
  members,
  loading,
  onRemove,
}: {
  members: string[];
  loading: boolean;
  onRemove: (email: string) => Promise<void>;
}) {
  if (loading) return <Loading text="Loading members…" />;
  if (members.length === 0) return <Empty text="No members yet." compact />;
  return (
    <div class="space-y-2">
      {members.map((m) => (
        <MemberRow key={m} email={m} onRemove={() => onRemove(m)} />
      ))}
    </div>
  );
}

function MemberRow({
  email,
  onRemove,
}: {
  email: string;
  onRemove: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const remove = async () => {
    if (!confirm(`Remove ${email} from this project?`)) return;
    setBusy(true);
    setErr(null);
    try {
      await onRemove();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2 space-y-1">
      <div class="flex items-center gap-2 min-w-0">
        <span class="text-[12.5px] text-ink-50 truncate" title={email}>
          {email}
        </span>
        <button
          type="button"
          onClick={remove}
          disabled={busy}
          class="h-7 w-7 ml-auto rounded text-ink-300 hover:text-accent-red hover:bg-white/[0.08] grid place-items-center disabled:opacity-50"
          aria-label={`Remove ${email}`}
          title="Remove member"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </div>
  );
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
          {info && <StateBadge state={info.state ?? "UNKNOWN"} />}
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

function InfoBody({
  project,
  record,
  onRepairNetwork,
}: {
  project: ProjectMeta;
  record: ProjectContainerRecord;
  onRepairNetwork: () => Promise<void>;
}) {
  if (record.error) {
    return (
      <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
        <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
        <div class="min-w-0">
          <div class="font-medium text-accent-red">Container endpoint unavailable</div>
          <div class="text-ink-200 mt-0.5 break-words">
            GET /api/projects/{project.id}/container returned {record.error}.
          </div>
        </div>
      </div>
    );
  }
  if (record.loading && !record.data) {
    return <Loading text="Loading container data…" />;
  }
  if (!record.data) return <Loading text="No data." />;

  const info = record.data;
  return (
    <>
      <Panel title="Overview">
        <Grid>
          <Field label="Container" value={info.name} mono />
          <Field label="State" value={info.state ?? "UNKNOWN"} mono />
          <Field label="PID" value={info.pid ? String(info.pid) : "—"} mono />
          <Field label="Processes" value={info.resources?.processes ? String(info.resources.processes) : "—"} mono />
          <Field label="Image" value={info.image || "—"} />
          <Field label="Architecture" value={info.architecture || "—"} mono />
          <Field label="Boot autostart" value={info.bootAutostart ? "yes" : "no"} mono />
          <Field label="Created" value={fmtDate(info.createdAt)} mono />
          <Field label="Last used" value={fmtDate(info.lastUsedAt)} mono />
        </Grid>
      </Panel>
      {info.os && <OSPanel os={info.os} />}
      {info.resources && <ResourcesPanel res={info.resources} />}
      {info.disks && info.disks.length > 0 && <DisksPanel disks={info.disks} />}
      {info.network && info.network.length > 0 && <NetworkPanel ifaces={info.network} onRepair={onRepairNetwork} />}
      {info.workspace && <WorkspacePanel ws={info.workspace} />}
      {info.limits && <LimitsPanel limits={info.limits} />}
      <ClaudePanel claude={info.claude} />
      <CodexPanel codex={info.codex} />
      {info.authBundles && info.authBundles.length > 0 && <AuthBundlesPanel bundles={info.authBundles} />}
    </>
  );
}

function SecretsBody({
  record,
  onSave,
  onDelete,
}: {
  record: SecretsRecord;
  onSave: (key: string, value: string) => Promise<void>;
  onDelete: (key: string) => Promise<void>;
}) {
  return (
    <>
      {record.error && (
        <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
          <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
          <div class="text-accent-red break-words">{record.error}</div>
        </div>
      )}
      <SecretEditor onSave={onSave} />
      <SecretsList list={record.data ?? []} loading={record.loading && !record.data} onSave={onSave} onDelete={onDelete} />
      <p class="text-[11.5px] text-ink-400 leading-relaxed">
        Secrets are passed to the selected agent CLI as <span class="font-mono">--env KEY=VALUE</span> on every prompt run. They never land in the container's filesystem and are not synced back from it.
      </p>
    </>
  );
}

function SecretEditor({
  onSave,
}: {
  onSave: (key: string, value: string) => Promise<void>;
}) {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async (e: Event) => {
    e.preventDefault();
    const k = key.trim();
    if (!k) {
      setErr("Key is required.");
      return;
    }
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)) {
      setErr("Key must match [A-Za-z_][A-Za-z0-9_]*");
      return;
    }
    setErr(null);
    setSubmitting(true);
    try {
      await onSave(k, value);
      setKey("");
      setValue("");
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={submit} class="rounded-md border border-white/10 bg-white/[0.03] p-2.5 space-y-2">
      <div class="grid gap-2 sm:grid-cols-[1fr_2fr_auto] items-start">
        <input
          value={key}
          onInput={(e) => setKey((e.target as HTMLInputElement).value)}
          placeholder="KEY"
          class="h-9 px-2.5 rounded border border-white/10 bg-black/30 text-[13px] font-mono text-ink-50 placeholder-ink-400 focus:outline-none focus:border-accent-blue/50"
        />
        <textarea
          value={value}
          onInput={(e) => setValue((e.target as HTMLTextAreaElement).value)}
          placeholder="value (multi-line OK — paste PEM keys, JSON, etc.)"
          rows={1}
          spellcheck={false}
          autoComplete="off"
          class="min-h-9 max-h-48 px-2.5 py-1.5 rounded border border-white/10 bg-black/30 text-[13px] font-mono text-ink-50 placeholder-ink-400 focus:outline-none focus:border-accent-blue/50 resize-y leading-[1.45] overflow-y-auto"
          style={{ fieldSizing: 'content' } as any}
        />
        <button
          type="submit"
          disabled={submitting}
          class="h-9 px-3 rounded bg-accent-blue/80 hover:bg-accent-blue text-white text-[13px] font-medium disabled:opacity-50"
        >
          {submitting ? "Saving…" : "Add"}
        </button>
      </div>
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </form>
  );
}

function SecretsList({
  list,
  loading,
  onSave,
  onDelete,
}: {
  list: ProjectSecret[];
  loading: boolean;
  onSave: (key: string, value: string) => Promise<void>;
  onDelete: (key: string) => Promise<void>;
}) {
  if (loading) return <Loading text="Loading secrets…" />;
  if (list.length === 0) return <Empty text="No secrets yet." compact />;
  return (
    <div class="space-y-2">
      {list.map((s) => (
        <SecretRow key={s.key} secret={s} onSave={onSave} onDelete={onDelete} />
      ))}
    </div>
  );
}

function SecretRow({
  secret,
  onSave,
  onDelete,
}: {
  secret: ProjectSecret;
  onSave: (key: string, value: string) => Promise<void>;
  onDelete: (key: string) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [draft, setDraft] = useState(secret.value);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const save = async () => {
    setBusy(true);
    setErr(null);
    try {
      await onSave(secret.key, draft);
      setEditing(false);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!confirm(`Delete secret ${secret.key}?`)) return;
    setBusy(true);
    try {
      await onDelete(secret.key);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2 space-y-1">
      <div class="flex items-center gap-2 min-w-0">
        <span class="font-mono text-[12.5px] text-ink-50 truncate">{secret.key}</span>
        <span class="text-[11px] text-ink-400 ml-auto whitespace-nowrap">
          updated {fmtMtime(secret.updatedAt)}
        </span>
      </div>
      {editing ? (
        <div class="flex items-start gap-2 flex-wrap">
          <textarea
            value={draft}
            onInput={(e) => setDraft((e.target as HTMLTextAreaElement).value)}
            rows={1}
            spellcheck={false}
            class="flex-1 min-h-8 max-h-48 px-2 py-1 rounded border border-white/10 bg-black/30 text-[12.5px] font-mono text-ink-50 focus:outline-none focus:border-accent-blue/50 resize-y leading-[1.45] overflow-y-auto"
            style={{ fieldSizing: 'content' } as any}
          />
          <button
            type="button"
            onClick={() => setRevealed(!revealed)}
            class="h-8 px-2 rounded text-[11px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.08]"
          >
            {revealed ? "hide" : "show"}
          </button>
          <button
            type="button"
            onClick={save}
            disabled={busy}
            class="h-8 px-2.5 rounded bg-accent-blue/80 hover:bg-accent-blue text-white text-[12px] font-medium disabled:opacity-50"
          >
            Save
          </button>
          <button
            type="button"
            onClick={() => {
              setEditing(false);
              setDraft(secret.value);
              setErr(null);
            }}
            class="h-8 px-2 rounded text-[12px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.08]"
          >
            Cancel
          </button>
        </div>
      ) : (
        <div class="flex items-center gap-2">
          <code class="flex-1 text-[12.5px] font-mono text-ink-100 break-all min-w-0 whitespace-pre-wrap max-h-48 overflow-y-auto">
            {revealed
              ? secret.value
              : hasNewlines(secret.value)
                ? lineSummary(secret.value)
                : "•".repeat(Math.min(20, secret.value.length || 6))}
          </code>
          <button
            type="button"
            onClick={() => setRevealed(!revealed)}
            class="h-7 px-2 rounded text-[11px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.08]"
          >
            {revealed ? "hide" : "show"}
          </button>
          <button
            type="button"
            onClick={() => setEditing(true)}
            class="h-7 px-2 rounded text-[11px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.08]"
          >
            edit
          </button>
          <button
            type="button"
            onClick={remove}
            disabled={busy}
            class="h-7 w-7 rounded text-ink-300 hover:text-accent-red hover:bg-white/[0.08] grid place-items-center disabled:opacity-50"
            aria-label="Delete"
            title="Delete"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
      )}
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </div>
  );
}

function StateBadge({ state }: { state: string }) {
  const tone =
    state === "RUNNING"
      ? "text-accent-green bg-accent-green/[0.12]"
      : state === "STOPPED"
      ? "text-ink-300 bg-white/[0.06]"
      : state === "MISSING"
      ? "text-accent-red bg-accent-red/[0.12]"
      : "text-ink-300 bg-white/[0.06]";
  return (
    <span class={`inline-flex items-center h-5 px-1.5 rounded text-[11px] font-medium ${tone}`}>
      {state.toLowerCase()}
    </span>
  );
}

function OSPanel({ os }: { os: OSInfo }) {
  return (
    <Panel title="OS">
      <Grid>
        <Field label="Distribution" value={os.prettyName || "—"} />
        <Field label="Kernel" value={os.kernel || "—"} mono />
        <Field label="Hostname" value={os.hostname || "—"} mono />
        <Field label="CPU count" value={os.cpuCount ? String(os.cpuCount) : "—"} mono />
        <Field label="Uptime" value={os.uptimeSec ? fmtDuration(os.uptimeSec) : "—"} mono />
      </Grid>
    </Panel>
  );
}

function ResourcesPanel({ res }: { res: ResourceInfo }) {
  return (
    <Panel title="Resources">
      <Grid>
        <Field label="Memory used" value={`${fmtBytes(res.memoryCurrentBytes)} / ${fmtBytes(res.memoryTotalBytes)}`} mono />
        <Field label="Memory peak" value={fmtBytes(res.memoryPeakBytes)} mono />
        <Field label="Swap" value={fmtBytes(res.swapCurrentBytes)} mono />
        <Field label="Disk (rootfs)" value={fmtBytes(res.diskUsageBytes)} mono />
        <Field label="CPU time" value={res.cpuUsageSeconds ? `${res.cpuUsageSeconds.toLocaleString()} s` : "—"} mono />
        <Field label="Processes" value={res.processes ? String(res.processes) : "—"} mono />
      </Grid>
    </Panel>
  );
}

function DisksPanel({ disks }: { disks: DiskUsage[] }) {
  return (
    <Panel title="Disk usage (inside container)">
      <div class="space-y-2">
        {disks.map((d) => {
          const pct = d.totalBytes && d.usedBytes != null ? Math.round((d.usedBytes / d.totalBytes) * 100) : null;
          return (
            <div
              key={d.mountPath}
              class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2"
            >
              <div class="flex items-center justify-between gap-2 min-w-0">
                <div class="font-mono text-[12.5px] text-ink-100 truncate">{d.mountPath}</div>
                <div class="text-[11px] text-ink-300 font-mono whitespace-nowrap">
                  {fmtBytes(d.usedBytes)} / {fmtBytes(d.totalBytes)}
                  {pct != null && <span class="ml-1.5 text-ink-400">({pct}%)</span>}
                </div>
              </div>
              {pct != null && (
                <div class="mt-1.5 h-1 rounded-full bg-white/[0.06] overflow-hidden">
                  <div
                    class={`h-full ${pct > 85 ? "bg-accent-red" : pct > 60 ? "bg-accent-orange" : "bg-accent-green"}`}
                    style={{ width: `${Math.min(100, pct)}%` }}
                  />
                </div>
              )}
              <div class="mt-1 text-[11px] text-ink-400 font-mono truncate">
                {d.filesystem} · avail {fmtBytes(d.availBytes)}
              </div>
            </div>
          );
        })}
      </div>
    </Panel>
  );
}

function NetworkPanel({ ifaces, onRepair }: { ifaces: NetworkInterface[]; onRepair: () => Promise<void> }) {
  const [repairing, setRepairing] = useState(false);
  const [repairErr, setRepairErr] = useState<string | null>(null);
  const noIPv4 = !ifaces.some((n) => (n.addresses ?? []).some((a) => a.split("/")[0].includes(".")));

  async function repair() {
    setRepairing(true);
    setRepairErr(null);
    try {
      await onRepair();
    } catch (e) {
      setRepairErr((e as Error).message);
    } finally {
      setRepairing(false);
    }
  }

  return (
    <Panel title="Network">
      <div class="space-y-2">
        {noIPv4 && (
          <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
            <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
            <div class="text-accent-red break-words">
              No IPv4 address — the container has no internet access. Use "Repair network" to re-run DHCP.
            </div>
          </div>
        )}
        {repairErr && (
          <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
            <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
            <div class="text-accent-red break-words">{repairErr}</div>
          </div>
        )}
        {ifaces.map((n) => (
          <div
            key={n.name}
            class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2 space-y-1"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-mono text-[12.5px] text-ink-100">{n.name}</span>
              <span class="text-[11px] text-ink-400">{n.state ?? "—"}</span>
              {n.macAddress && (
                <span class="ml-auto font-mono text-[11px] text-ink-400">{n.macAddress}</span>
              )}
            </div>
            {n.addresses && n.addresses.length > 0 && (
              <div class="font-mono text-[12px] text-ink-200 break-all">
                {n.addresses.join(", ")}
              </div>
            )}
            <div class="text-[11px] text-ink-400 font-mono">
              rx {fmtBytes(n.bytesReceived)} · tx {fmtBytes(n.bytesSent)}
              {n.mtu ? ` · mtu ${n.mtu}` : ""}
              {n.hostName ? ` · host ${n.hostName}` : ""}
            </div>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={() => void repair()}
        disabled={repairing}
        class="mt-2 h-9 w-full rounded-md border border-white/10 bg-white/[0.04] px-3 text-[13px] font-medium text-ink-100 hover:bg-white/[0.08] disabled:opacity-45 disabled:cursor-not-allowed"
        title="Re-runs DHCP on eth0 inside the container. Fixes the 'running but no internet' state."
      >
        {repairing ? "Repairing network..." : "Repair network"}
      </button>
    </Panel>
  );
}

function WorkspacePanel({ ws }: { ws: WorkspaceInfo }) {
  return (
    <Panel title="Workspace mount">
      <Grid>
        <Field label="Host source" value={ws.hostSource || "—"} mono />
        <Field label="Container path" value={ws.containerPath || "—"} mono />
      </Grid>
    </Panel>
  );
}

function LimitsPanel({ limits }: { limits: ContainerLimits }) {
  return (
    <Panel title="Resource limits">
      <Grid>
        <Field label="CPU" value={limits.cpu || "—"} mono />
        <Field label="Memory" value={limits.memory || "—"} mono />
        <Field label="Disk" value={limits.disk || "—"} mono />
      </Grid>
    </Panel>
  );
}

function ClaudePanel({ claude }: { claude: ProjectContainerInfo["claude"] }) {
  return (
    <Panel title="Claude provisioning">
      <Grid>
        <Field label="CLI installed" value={claude.installed ? "yes" : "no"} mono />
        <Field label="Version" value={claude.version || "—"} mono />
        <Field label="CLAUDE.md" value={claude.claudeMdInstalled ? "installed" : "missing"} mono />
        <Field
          label="CLAUDE.md in sync"
          value={claude.claudeMdInSync ? "yes" : "no"}
          mono
          tone={claude.claudeMdInstalled && !claude.claudeMdInSync ? "warn" : undefined}
        />
      </Grid>
    </Panel>
  );
}

function CodexPanel({ codex }: { codex: ProjectContainerInfo["codex"] }) {
  return (
    <Panel title="Codex provisioning">
      <Grid>
        <Field label="CLI installed" value={codex.installed ? "yes" : "no"} mono />
        <Field label="Version" value={codex.version || "—"} mono />
      </Grid>
    </Panel>
  );
}

function AuthBundlesPanel({ bundles }: { bundles: AuthBundleStatus[] }) {
  return (
    <Panel title="Auth bundles">
      <div class="space-y-3">
        {bundles.map((b) => (
          <div key={b.name} class="rounded-md border border-white/[0.08] bg-white/[0.03] p-2.5 space-y-2">
            <div class="text-[12.5px] font-semibold text-ink-100">{b.name}</div>
            <div class="space-y-1.5">
              {b.files.map((f) => (
                <AuthFileRow key={f.containerPath} f={f} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </Panel>
  );
}

function AuthFileRow({ f }: { f: AuthBundleFileStatus }) {
  const tone = f.containerNewer
    ? "text-accent-orange"
    : f.hostNewer
    ? "text-ink-300"
    : f.hostExists && f.containerExists
    ? "text-accent-green"
    : "text-ink-400";
  const label = f.containerNewer
    ? "container rotated — pending pull"
    : f.hostNewer
    ? "host newer — will push next prompt"
    : f.hostExists && f.containerExists
    ? "in sync"
    : !f.hostExists && !f.containerExists
    ? "missing on both"
    : f.hostExists
    ? "host only"
    : "container only";
  return (
    <div class="rounded border border-white/[0.06] bg-black/20 px-2.5 py-1.5">
      <div class="font-mono text-[11.5px] text-ink-200 break-all">{f.containerPath}</div>
      <div class={`text-[11px] mt-0.5 ${tone}`}>{label}</div>
      <div class="text-[10.5px] font-mono text-ink-400 mt-0.5">
        host {f.hostExists ? fmtMtime(f.hostMtime) : "—"} · container {f.containerExists ? fmtMtime(f.containerMtime) : "—"}
      </div>
    </div>
  );
}

function Loading({ text }: { text: string }) {
  return (
    <div class="rounded-md border border-white/10 bg-white/[0.03] px-3 py-4 text-center text-[12.5px] text-ink-300">
      {text}
    </div>
  );
}

function Empty({ text, compact }: { text: string; compact?: boolean }) {
  return (
    <div
      class={`rounded-lg border border-white/10 bg-[#101318] ${
        compact ? "px-3 py-2.5" : "px-4 py-5"
      } text-sm text-ink-300`}
    >
      {text}
    </div>
  );
}

function Panel({ title, children }: { title: string; children: JSX.Element | JSX.Element[] }) {
  return (
    <section class="rounded-md border border-white/[0.08] bg-white/[0.02] overflow-hidden">
      <header class="px-3 py-2 border-b border-white/[0.06]">
        <h3 class="text-[11.5px] font-semibold uppercase tracking-wide text-ink-300">{title}</h3>
      </header>
      <div class="p-2.5">{children}</div>
    </section>
  );
}

function Grid({ children }: { children: JSX.Element | JSX.Element[] }) {
  return <div class="grid gap-2 sm:grid-cols-2">{children}</div>;
}

function Field({
  label,
  value,
  mono,
  tone,
}: {
  label: string;
  value: string;
  mono?: boolean;
  tone?: "warn";
}) {
  return (
    <div class="rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-2 min-w-0">
      <div class="text-[11px] text-ink-400">{label}</div>
      <div
        class={`mt-0.5 text-[12.5px] truncate ${mono ? "font-mono" : ""} ${
          tone === "warn" ? "text-accent-orange" : "text-ink-100"
        }`}
        title={value}
      >
        {value}
      </div>
    </div>
  );
}

function fmtBytes(n?: number): string {
  if (n == null || !isFinite(n)) return "—";
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB", "PB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 2 : v < 100 ? 1 : 0)} ${units[i]}`;
}

function fmtDate(iso?: string): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function fmtMtime(unix?: number): string {
  if (!unix) return "—";
  try {
    return new Date(unix * 1000).toLocaleString();
  } catch {
    return String(unix);
  }
}

function fmtDuration(secs: number): string {
  if (!secs || secs < 0) return "—";
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const parts: string[] = [];
  if (d) parts.push(`${d}d`);
  if (h) parts.push(`${h}h`);
  if (m && !d) parts.push(`${m}m`);
  if (parts.length === 0) parts.push(`${Math.floor(secs)}s`);
  return parts.join(" ");
}

function fmtRelative(ts: number): string {
  const s = Math.floor((Date.now() - ts) / 1000);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  return `${Math.floor(s / 3600)}h ago`;
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + "…" : s;
}


function hasNewlines(v: string): boolean {
  for (let i = 0; i < v.length; i++) if (v.charCodeAt(i) === 10) return true;
  return false;
}

function lineSummary(v: string): string {
  let lines = 1;
  for (let i = 0; i < v.length; i++) if (v.charCodeAt(i) === 10) lines++;
  return '• ' + lines + ' lines •';
}
