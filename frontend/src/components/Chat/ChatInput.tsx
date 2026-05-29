import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { uploadChatFiles } from "../../lib/api";
import type { ChatMode } from "../../types";
import { ArrowUp, Clock, File as FileIcon, Plus, Square, Upload, X } from "../icons";

interface Props {
  chatId: string;
  streaming: boolean;
  canSendPrompt: boolean;
  model: string;
  mode: ChatMode;
  queuedPrompts: Array<{ id: string; text: string }>;
  draftText?: string;
  draftKey?: number;
  onModelChange: (model: string) => void;
  onModeChange: (mode: ChatMode) => void;
  onRemoveQueued: (id: string) => void;
  onSend: (text: string) => boolean;
  onCancel: () => void;
  onAfterUpload?: () => void;
}

interface Attachment {
  id: string;
  name: string;
  size: number;
  serverPath: string;
  isImage: boolean;
  objectUrl?: string;
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n}B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function randomId(): string {
  return Math.random().toString(36).slice(2, 10);
}

const MODEL_OPTIONS = [
  { value: "", label: "Auto" },
  { value: "opus", label: "Opus" },
  { value: "sonnet", label: "Sonnet" },
  { value: "haiku", label: "Haiku" },
];

const MODE_OPTIONS: Array<{ value: ChatMode; label: string }> = [
  { value: "chat", label: "Chat" },
  { value: "plan", label: "Plan" },
  { value: "code", label: "Code" },
  { value: "review", label: "Review" },
  { value: "debug", label: "Debug" },
  { value: "full-auto", label: "Full auto" },
];

export function ChatInput({
  chatId,
  streaming,
  canSendPrompt,
  model,
  mode,
  queuedPrompts,
  draftText,
  draftKey,
  onModelChange,
  onModeChange,
  onRemoveQueued,
  onSend,
  onCancel,
  onAfterUpload,
}: Props) {
  const [text, setText] = useState("");
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);

  useEffect(() => {
    const ta = taRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = Math.min(ta.scrollHeight, 220) + "px";
  }, [text]);

  useEffect(() => {
    if (draftKey === undefined) return;
    setText(draftText ?? "");
    setTimeout(focusInput, 0);
  }, [draftKey, draftText]);

  // Clear attachments when switching chats.
  useEffect(() => {
    setAttachments((prev) => {
      prev.forEach((a) => { if (a.objectUrl) URL.revokeObjectURL(a.objectUrl); });
      return [];
    });
    setText("");
  }, [chatId]);

  // Revoke any remaining object URLs on unmount.
  useEffect(() => () => {
    setAttachments((prev) => {
      prev.forEach((a) => { if (a.objectUrl) URL.revokeObjectURL(a.objectUrl); });
      return [];
    });
  }, []);

  function focusInput() {
    const ta = taRef.current;
    if (!ta) return;
    ta.focus();
    const end = ta.value.length;
    ta.setSelectionRange(end, end);
  }

  const doUpload = useCallback(async (files: File[]) => {
    if (!files.length) return;
    // Capture client-side previews + sizes BEFORE upload so the chips render
    // instantly. Replace serverPath with the real one when the response lands.
    const localized: Attachment[] = files.map((f) => ({
      id: randomId(),
      name: f.name,
      size: f.size,
      serverPath: "",
      isImage: f.type.startsWith("image/"),
      objectUrl: f.type.startsWith("image/") ? URL.createObjectURL(f) : undefined,
    }));
    setAttachments((prev) => [...prev, ...localized]);
    setUploading(true);
    try {
      const res = await uploadChatFiles(chatId, files);
      // Pair upload results back to the freshly-added chips by name + order.
      // Backend returns results in the same order as inputs.
      setAttachments((prev) => {
        const next = [...prev];
        for (let i = 0; i < res.results.length; i++) {
          const r = res.results[i];
          const localIdx = next.findIndex((a) => a.id === localized[i].id);
          if (localIdx < 0) continue;
          if (r.error) {
            // Drop failed uploads, free their object URL.
            const a = next[localIdx];
            if (a.objectUrl) URL.revokeObjectURL(a.objectUrl);
            next.splice(localIdx, 1);
          } else {
            next[localIdx] = { ...next[localIdx], serverPath: r.path || "" };
          }
        }
        return next;
      });
      const failed = res.results.filter((r) => r.error);
      if (failed.length) {
        alert("Failed:\n" + failed.map((f) => `${f.name} — ${f.error}`).join("\n"));
      }
      onAfterUpload?.();
    } catch (e) {
      // On total failure, remove all the just-added chips.
      setAttachments((prev) => {
        const ids = new Set(localized.map((l) => l.id));
        prev.forEach((a) => { if (ids.has(a.id) && a.objectUrl) URL.revokeObjectURL(a.objectUrl); });
        return prev.filter((a) => !ids.has(a.id));
      });
      alert("upload failed: " + (e as Error).message);
    } finally {
      setUploading(false);
    }
  }, [chatId, onAfterUpload]);

  function removeAttachment(id: string) {
    setAttachments((prev) => {
      const t = prev.find((a) => a.id === id);
      if (t?.objectUrl) URL.revokeObjectURL(t.objectUrl);
      return prev.filter((a) => a.id !== id);
    });
  }

  function send() {
    if (uploading || (!streaming && !canSendPrompt)) return;
    const userText = text.trim();
    // Only include attachments that finished uploading.
    const paths = attachments.filter((a) => a.serverPath).map((a) => a.serverPath);
    if (!userText && paths.length === 0) return;
    const finalText = paths.length
      ? (userText ? `${userText}\n\n${paths.join(" ")}` : paths.join(" "))
      : userText;
    if (!onSend(finalText)) return;
    setText("");
    // Clear attachments + free previews
    attachments.forEach((a) => { if (a.objectUrl) URL.revokeObjectURL(a.objectUrl); });
    setAttachments([]);
    setTimeout(focusInput, 0);
  }

  // -- drag/drop ----------------------------------------------------------
  useEffect(() => {
    function onDragEnter(e: DragEvent) {
      if (!e.dataTransfer || !Array.from(e.dataTransfer.types).includes("Files")) return;
      e.preventDefault();
      dragCounter.current++;
      setDragging(true);
    }
    function onDragOver(e: DragEvent) {
      if (!e.dataTransfer || !Array.from(e.dataTransfer.types).includes("Files")) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
    }
    function onDragLeave(e: DragEvent) {
      if (!e.dataTransfer || !Array.from(e.dataTransfer.types).includes("Files")) return;
      dragCounter.current = Math.max(0, dragCounter.current - 1);
      if (dragCounter.current === 0) setDragging(false);
    }
    function onDrop(e: DragEvent) {
      if (!e.dataTransfer) return;
      const files = Array.from(e.dataTransfer.files);
      if (!files.length) return;
      e.preventDefault();
      dragCounter.current = 0;
      setDragging(false);
      doUpload(files);
    }
    window.addEventListener("dragenter", onDragEnter);
    window.addEventListener("dragover", onDragOver);
    window.addEventListener("dragleave", onDragLeave);
    window.addEventListener("drop", onDrop);
    return () => {
      window.removeEventListener("dragenter", onDragEnter);
      window.removeEventListener("dragover", onDragOver);
      window.removeEventListener("dragleave", onDragLeave);
      window.removeEventListener("drop", onDrop);
    };
  }, [doUpload]);

  function onPaste(e: ClipboardEvent) {
    const items = e.clipboardData?.items;
    if (!items) return;
    const files: File[] = [];
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      if (it.kind === "file") {
        const f = it.getAsFile();
        if (f) files.push(f);
      }
    }
    if (files.length) {
      e.preventDefault();
      doUpload(files);
    }
  }

  const disconnected = !canSendPrompt && !streaming;
  const hasContent = text.trim().length > 0 || attachments.some((a) => a.serverPath);
  const canSend = !uploading && !disconnected && hasContent;

  return (
    <div class="codex-composer-shell flex-none z-20 relative bg-[#0b0d11] border-t border-white/10">
      {dragging && (
        <div class="absolute inset-x-3 -top-16 z-20 rounded-lg border-2 border-dashed border-accent-blue
                    bg-[#151922] text-accent-blue text-sm flex items-center justify-center
                    h-14 gap-2">
          <Upload class="w-5 h-5" />
          Drop files to upload to the chat directory
        </div>
      )}

      <div class="codex-composer-controls px-3 pt-2 pb-1.5">
        <div class="mx-auto max-w-[980px] flex items-center gap-2 overflow-x-auto no-scrollbar">
          <label class="codex-model-control hidden sm:inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
            <span class="hidden sm:inline text-ink-400">Model</span>
            <select
              value={model}
              onChange={(e) => onModelChange((e.currentTarget as HTMLSelectElement).value)}
              class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
              title="Model"
            >
              {model && !MODEL_OPTIONS.some((opt) => opt.value === model) && (
                <option value={model}>{model}</option>
              )}
              {MODEL_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </label>

          <label class="codex-mode-control inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
            <span class="hidden sm:inline text-ink-400">Mode</span>
            <select
              value={mode}
              onChange={(e) => onModeChange((e.currentTarget as HTMLSelectElement).value as ChatMode)}
              class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
              title="Mode"
            >
              {MODE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </label>

          {streaming && (
            <div class="inline-flex items-center gap-1.5 h-9 px-2.5 rounded-md bg-accent-blue/[0.12] border border-accent-blue/25 text-[12px] text-accent-blue flex-none">
              <Clock class="w-3.5 h-3.5" />
              <span class="hidden sm:inline">Next send queues</span>
              <span class="sm:hidden">Queues</span>
            </div>
          )}
        </div>
      </div>

      {queuedPrompts.length > 0 && (
        <div class="px-3 pb-2">
          <div class="mx-auto max-w-[980px] rounded-lg border border-white/10 bg-white/[0.035] p-2">
            <div class="flex items-center justify-between gap-3 px-1 pb-1.5">
              <div class="text-[12px] font-medium text-ink-200">Queue</div>
              <div class="text-[11px] text-ink-400">{queuedPrompts.length} waiting</div>
            </div>
            <div class="flex flex-wrap gap-2">
              {queuedPrompts.map((q, index) => (
                <div key={q.id} class="group min-w-0 max-w-full inline-flex items-center gap-2 rounded-md bg-[#101318] border border-white/10 px-2 py-1.5">
                  <span class="text-[11px] text-ink-400 flex-none">#{index + 1}</span>
                  <span class="text-[12px] text-ink-100 truncate max-w-[260px] sm:max-w-[420px]" title={q.text}>
                    {q.text}
                  </span>
                  <button
                    type="button"
                    onClick={() => onRemoveQueued(q.id)}
                    class="w-6 h-6 grid place-items-center rounded text-ink-300 hover:text-accent-red hover:bg-accent-red/10 flex-none"
                    aria-label="Remove queued prompt"
                    title="Remove queued prompt"
                  >
                    <X class="w-3 h-3" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {attachments.length > 0 && (
        <div class="px-3 pb-1 flex flex-wrap gap-2 max-h-[180px] overflow-y-auto touch-scroll scrollbar-thin">
          {attachments.map((a) => (
            <AttachmentChip key={a.id} att={a} onRemove={() => removeAttachment(a.id)} />
          ))}
        </div>
      )}

      <form
        onSubmit={(e) => { e.preventDefault(); send(); }}
        class="codex-composer-form composer-form flex gap-2 items-end px-3 pt-2"
      >
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          disabled={uploading || disconnected}
          class="codex-icon-button flex-none w-11 h-11 rounded-md bg-white/[0.06] border border-white/10
                 hover:bg-white/10 active:bg-accent-blue active:border-accent-blue active:scale-[0.98]
                 disabled:opacity-50 disabled:active:scale-100 grid place-items-center text-ink-100 transition"
          aria-label="Attach files"
          title="Attach (or drag-and-drop / paste images)"
        >
          <Plus class="w-4 h-4" />
        </button>
        <input
          ref={fileRef}
          type="file"
          multiple
          class="hidden"
          onChange={(e) => {
            const inp = e.currentTarget as HTMLInputElement;
            const files = Array.from(inp.files || []);
            inp.value = "";
            doUpload(files);
          }}
        />
        <textarea
          ref={taRef}
          value={text}
          onInput={(e) => setText((e.currentTarget as HTMLTextAreaElement).value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
              e.preventDefault();
              send();
            }
          }}
          onPaste={onPaste}
          rows={1}
          enterkeyhint="send"
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
          placeholder={
            uploading ? "Uploading…" :
            streaming ? "Queue next prompt while Claude is working" :
            disconnected ? "Connecting…" :
            "Ask Codex anything, @ to add files, / for commands"
          }
          disabled={disconnected}
          class="codex-composer-textarea flex-1 resize-none rounded-md
                 bg-[#101318] border border-white/10 text-ink-100 placeholder:text-ink-300
                 focus:outline-none focus:border-accent-blue/80 focus:bg-[#121722]
                 px-3.5 py-3 text-[16px] sm:text-[14.5px] leading-normal
                 min-h-[44px] max-h-[220px] shadow-inner
                 disabled:opacity-60 transition-colors"
        />
        <button
          type="submit"
          disabled={!canSend}
          class={`codex-send-button flex-none w-11 h-11 rounded-md
                  ${streaming ? "bg-accent-blue hover:bg-accent-blue/85" : "bg-accent-green hover:bg-accent-green/85"}
                  disabled:bg-ink-500 disabled:cursor-not-allowed
                  active:scale-[0.98] disabled:active:scale-100 grid place-items-center text-white transition`}
          aria-label={streaming ? "Queue prompt" : "Send"}
          title={canSend ? (streaming ? "Queue prompt" : "Send") : disconnected ? "Connecting" : "Send"}
        >
          {streaming ? <Clock class="w-4 h-4" /> : <ArrowUp class="w-4 h-4" />}
        </button>
        {streaming && (
          <button
            type="button"
            onClick={onCancel}
            class="codex-cancel-button flex-none w-11 h-11 rounded-md bg-accent-red/90 hover:bg-accent-red
                   active:scale-[0.98] grid place-items-center text-white transition"
            aria-label="Cancel"
            title="Cancel current generation"
          >
            <Square class="w-3.5 h-3.5" />
          </button>
        )}
      </form>
    </div>
  );
}

function AttachmentChip({ att, onRemove }: { att: Attachment; onRemove: () => void }) {
  const pending = !att.serverPath;
  if (att.isImage && att.objectUrl) {
    return (
      <div class="relative w-20 h-20 rounded-lg overflow-hidden bg-[#101318] border border-white/10 group">
        <img src={att.objectUrl} class="w-full h-full object-cover" alt={att.name} />
        {pending && (
          <div class="absolute inset-0 bg-black/40 grid place-items-center">
            <div class="w-5 h-5 border-2 border-white/70 border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        <button
          type="button"
          onClick={onRemove}
          class="absolute top-1 right-1 w-6 h-6 rounded-md bg-black/70 hover:bg-accent-red text-white grid place-items-center opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
          aria-label={`Remove ${att.name}`}
        >
          <X class="w-3 h-3" />
        </button>
        <div class="absolute bottom-0 left-0 right-0 px-1.5 py-0.5 bg-gradient-to-t from-black/85 to-transparent text-white text-[9.5px] truncate">
          {att.name}
        </div>
      </div>
    );
  }
  return (
    <div class="group flex items-center gap-1.5 bg-[#101318] border border-white/10 rounded-md px-2 py-1.5 text-xs min-h-10">
      {pending ? (
        <div class="w-3.5 h-3.5 border-2 border-ink-300 border-t-transparent rounded-full animate-spin flex-none" />
      ) : (
        <FileIcon class="w-3.5 h-3.5 text-accent-blue flex-none" />
      )}
      <span class="truncate max-w-[180px] text-ink-100" title={att.name}>{att.name}</span>
      <span class="text-ink-300 text-[10px] flex-none">{fmtBytes(att.size)}</span>
      <button
        type="button"
        onClick={onRemove}
        class="w-6 h-6 grid place-items-center rounded text-ink-300 hover:text-accent-red hover:bg-accent-red/10 flex-none -mr-1"
        aria-label={`Remove ${att.name}`}
      >
        <X class="w-3 h-3" />
      </button>
    </div>
  );
}
