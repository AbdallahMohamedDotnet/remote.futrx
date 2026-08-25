import { useEffect, useRef, useState } from "preact/hooks";
import { AlertCircle, Loader, X } from "../primitives/icons";

export function DeleteProjectModal({
  open,
  projectName,
  onClose,
  onDelete,
}: {
  open: boolean;
  projectName: string;
  onClose: () => void;
  onDelete: () => Promise<void>;
}) {
  const [confirmation, setConfirmation] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    setConfirmation("");
    setDeleting(false);
    setDeleteError("");
    const timer = setTimeout(() => inputRef.current?.focus(), 60);
    return () => clearTimeout(timer);
  }, [open, projectName]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  if (!open) return null;

  const matches = confirmation === projectName;

  function close() {
    if (!deleting) onClose();
  }

  async function submit() {
    if (!matches || deleting) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await onDelete();
      onClose();
    } catch (error) {
      setDeleteError("Delete failed: " + (error as Error).message);
      setDeleting(false);
    }
  }

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-8">
      <div
        class="absolute inset-0 bg-black/70 backdrop-blur-[3px] modal-backdrop-fade"
        onClick={close}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="delete-project-title"
        class="theme-menu-surface modal-card-pop relative w-full max-w-[480px] overflow-hidden rounded-[14px] border border-white/10 bg-ink-800 text-ink-50 shadow-[0_24px_64px_rgba(0,0,0,.6)]"
      >
        <div class="flex items-start justify-between gap-4 px-5 pb-3.5 pt-[18px]">
          <div class="flex flex-col gap-[3px]">
            <div id="delete-project-title" class="text-[15px] font-semibold tracking-[-0.01em]">
              Delete project
            </div>
            <div class="text-[12.5px] text-ink-300">
              This action cannot be undone.
            </div>
          </div>
          <button
            type="button"
            onClick={close}
            disabled={deleting}
            aria-label="Close"
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] text-ink-300 transition-colors hover:bg-white/5 hover:text-ink-100 disabled:opacity-45"
          >
            <X class="h-4 w-4" />
          </button>
        </div>

        <div class="flex flex-col gap-4 border-t border-white/10 p-5">
          <div class="rounded-[10px] border border-accent-red/25 bg-accent-red/[0.07] px-3.5 py-3 text-[13px] leading-5 text-ink-200">
            The <span class="font-mono text-ink-50">{projectName}</span> container, project settings, and associated chats will be permanently removed.
          </div>

          <div class="flex flex-col gap-[7px]">
            <label for="delete-project-confirmation" class="text-xs text-ink-300">
              Type <span class="font-mono text-ink-100">{projectName}</span> to confirm
            </label>
            <input
              id="delete-project-confirmation"
              ref={inputRef}
              value={confirmation}
              onInput={(event) => {
                setConfirmation((event.target as HTMLInputElement).value);
                setDeleteError("");
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submit();
              }}
              autocomplete="off"
              spellcheck={false}
              disabled={deleting}
              class="theme-submenu-surface w-full rounded-[9px] border border-white/[0.12] bg-[#101116] px-3 py-2.5 font-mono text-sm text-ink-100 outline-none transition-[border-color,box-shadow] duration-150 focus:border-accent-red/60 focus:shadow-[0_0_0_3px_rgba(255,123,114,.12)]"
            />
          </div>

          {deleteError && (
            <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
              <AlertCircle class="mt-0.5 h-4 w-4 flex-none text-accent-red" />
              <div class="break-words text-accent-red">{deleteError}</div>
            </div>
          )}
        </div>

        <div class="flex items-center justify-end gap-2 border-t border-white/10 bg-white/[0.02] px-5 py-3.5">
          <button
            type="button"
            onClick={close}
            disabled={deleting}
            class="rounded-lg border border-white/[0.12] px-3.5 py-2 text-[13px] text-ink-200 transition-colors hover:bg-white/5 hover:text-ink-100 disabled:opacity-45"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => void submit()}
            disabled={!matches || deleting}
            class="inline-flex items-center gap-[7px] rounded-lg border border-accent-red/40 bg-accent-red px-[15px] py-2 text-[13px] font-semibold text-ink-900 transition-colors hover:bg-accent-red/90 disabled:cursor-not-allowed disabled:border-white/[0.08] disabled:bg-white/[0.07] disabled:text-ink-400"
          >
            {deleting && <Loader class="h-3.5 w-3.5 animate-spin" />}
            {deleting ? "Deleting…" : "Delete project"}
          </button>
        </div>
      </div>
    </div>
  );
}
