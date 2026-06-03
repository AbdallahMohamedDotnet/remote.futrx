// LoginsPanel: surfaces every project secret named STORAGE_STATE_* as a
// reusable "login" the user can re-capture or delete. Wired into the
// ProjectContainersPage as a 4th collapsible section (Info → Secrets →
// Sharing → Logins). Opens LoginSessionModal for capture / re-capture.

import { useMemo, useState } from "preact/hooks";
import type { ProjectSecret } from "../../models/project";
import { LoginSessionModal } from "./LoginSessionModal";
import type { CaptureResult } from "../../services/loginSessionService";
import { Plus, RotateCcw, X, Key } from "../ui/icons";

const PREFIX = "STORAGE_STATE_";

export interface LoginsPanelProps {
  projectId: string | null;
  secrets: ProjectSecret[] | undefined;
  loading: boolean;
  error?: string;
  onDelete: (key: string) => Promise<void>;
  onSecretsChanged: () => void;
}

export function LoginsPanel({
  projectId,
  secrets,
  loading,
  error,
  onDelete,
  onSecretsChanged,
}: LoginsPanelProps) {
  const [modalOpen, setModalOpen] = useState(false);
  const [recaptureName, setRecaptureName] = useState<string | null>(null);

  const logins = useMemo(
    () => (secrets ?? []).filter((s) => s.key.startsWith(PREFIX)),
    [secrets]
  );

  const handleAdd = () => {
    setRecaptureName(null);
    setModalOpen(true);
  };

  const handleRecapture = (key: string) => {
    setRecaptureName(key.slice(PREFIX.length));
    setModalOpen(true);
  };

  const handleClose = () => {
    setModalOpen(false);
    setRecaptureName(null);
  };

  const handleCaptured = (_result: CaptureResult) => {
    onSecretsChanged();
  };

  return (
    <>
      {loading && !secrets && (
        <div class="text-[12px] text-ink-300">Loading…</div>
      )}
      {error && (
        <div class="text-[12px] text-red-300 bg-red-900/30 border border-red-700/40 rounded px-2 py-1">
          {error}
        </div>
      )}
      {!loading && logins.length === 0 && (
        <div class="text-[12px] text-ink-300">
          No captured logins yet. Click "Add login" to capture cookies + localStorage
          for an external site as a Playwright storageState secret.
        </div>
      )}
      {logins.length > 0 && (
        <ul class="space-y-1.5">
          {logins.map((s) => (
            <li
              key={s.key}
              class="flex items-center gap-2 rounded border border-white/[0.06] bg-white/[0.02] px-2 py-1.5"
            >
              <Key class="w-3.5 h-3.5 text-ink-300 flex-none" />
              <div class="flex-1 min-w-0">
                <div class="text-[12.5px] text-ink-50 font-mono truncate">{s.key}</div>
                <div class="text-[10.5px] text-ink-400">
                  {formatBytes(byteLength(s.value))} · updated {formatTime(s.updatedAt)}
                </div>
              </div>
              <button
                type="button"
                onClick={() => handleRecapture(s.key)}
                class="h-7 px-2 rounded text-ink-200 hover:bg-white/10 text-[11.5px] flex items-center gap-1"
                title="Re-capture"
                disabled={!projectId}
              >
                <RotateCcw class="w-3 h-3" /> Re-capture
              </button>
              <button
                type="button"
                onClick={() => {
                  if (!confirm(`Delete login secret ${s.key}?`)) return;
                  void onDelete(s.key);
                }}
                class="h-7 w-7 grid place-items-center rounded text-ink-200 hover:bg-red-900/40 hover:text-red-300"
                title="Delete"
                disabled={!projectId}
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div class="flex justify-end">
        <button
          type="button"
          onClick={handleAdd}
          disabled={!projectId}
          class="h-8 px-3 rounded bg-accent-blue/80 hover:bg-accent-blue text-white text-[12px] flex items-center gap-1.5 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Plus class="w-3.5 h-3.5" /> Add login
        </button>
      </div>

      {projectId && (
        <LoginSessionModal
          projectId={projectId}
          open={modalOpen}
          initialName={recaptureName ?? ""}
          onClose={handleClose}
          onCaptured={handleCaptured}
        />
      )}
    </>
  );
}

// byteLength: an approximate JSON byte length for display only.
function byteLength(s: string): number {
  // 1 char ~= 1 byte for ASCII JSON; close enough for a status line.
  return s.length;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTime(ts: number): string {
  if (!ts) return "—";
  const d = new Date(ts);
  const now = Date.now();
  const diff = now - ts;
  if (diff < 60_000) return "just now";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return d.toLocaleDateString();
}
