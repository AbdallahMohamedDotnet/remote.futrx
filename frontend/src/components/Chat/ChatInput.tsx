import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { uploadChatFiles } from "../../lib/api";
import { ArrowUp, File as FileIcon, Plus, Square, Upload, X } from "../icons";

interface Props {
  chatId: string;
  streaming: boolean;
  onSend: (text: string) => void;
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

export function ChatInput({ chatId, streaming, onSend, onCancel, onAfterUpload }: Props) {
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
    if (streaming || uploading) return;
    const userText = text.trim();
    // Only include attachments that finished uploading.
    const paths = attachments.filter((a) => a.serverPath).map((a) => a.serverPath);
    if (!userText && paths.length === 0) return;
    const finalText = paths.length
      ? (userText ? `${userText}\n\n${paths.join(" ")}` : paths.join(" "))
      : userText;
    onSend(finalText);
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

  const canSend = !streaming && !uploading && (text.trim().length > 0 || attachments.some((a) => a.serverPath));

  return (
    <div class="relative">
      {dragging && (
        <div class="absolute inset-x-2 -top-16 z-20 rounded-md border-2 border-dashed border-accent-blue
                    bg-ink-700/95 text-accent-blue text-sm flex items-center justify-center
                    h-14 gap-2 backdrop-blur-sm">
          <Upload class="w-5 h-5" />
          Drop files to upload to the chat directory
        </div>
      )}

      {attachments.length > 0 && (
        <div class="px-2 pt-2 pb-1 bg-ink-800 border-t border-ink-500 flex flex-wrap gap-2 max-h-[180px] overflow-y-auto scrollbar-thin">
          {attachments.map((a) => (
            <AttachmentChip key={a.id} att={a} onRemove={() => removeAttachment(a.id)} />
          ))}
        </div>
      )}

      <form
        onSubmit={(e) => { e.preventDefault(); send(); }}
        class={`flex gap-2 items-end p-2 bg-ink-800 ${attachments.length === 0 ? "border-t border-ink-500" : ""}`}
        style={{ paddingBottom: "calc(8px + env(safe-area-inset-bottom, 0px))" }}
      >
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          disabled={uploading || streaming}
          class="flex-none w-9 h-9 rounded-md bg-ink-600 border border-ink-500
                 hover:bg-ink-500 active:bg-accent-blue active:border-accent-blue
                 disabled:opacity-50 grid place-items-center text-ink-100"
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
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
          placeholder={
            uploading ? "Uploading…" :
            streaming ? "Claude is thinking… (Esc to cancel)" :
            "Message Claude — Enter to send, Shift+Enter for newline"
          }
          disabled={streaming}
          class="flex-1 resize-none rounded-md
                 bg-ink-700 border border-ink-500 text-ink-100 placeholder:text-ink-300
                 focus:outline-none focus:border-accent-blue
                 px-3 py-2 text-[14px] leading-normal
                 min-h-[36px] max-h-[220px]
                 disabled:opacity-60"
        />
        {streaming ? (
          <button
            type="button"
            onClick={onCancel}
            class="flex-none w-9 h-9 rounded-md bg-accent-red/90 hover:bg-accent-red
                   grid place-items-center text-white"
            aria-label="Cancel"
            title="Cancel current generation"
          >
            <Square class="w-3.5 h-3.5" />
          </button>
        ) : (
          <button
            type="submit"
            disabled={!canSend}
            class="flex-none w-9 h-9 rounded-md bg-accent-green hover:bg-accent-green/85
                   disabled:bg-ink-500 disabled:cursor-not-allowed
                   grid place-items-center text-white"
            aria-label="Send"
          >
            <ArrowUp class="w-4 h-4" />
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
      <div class="relative w-20 h-20 rounded-md overflow-hidden bg-ink-700 border border-ink-500 group">
        <img src={att.objectUrl} class="w-full h-full object-cover" alt={att.name} />
        {pending && (
          <div class="absolute inset-0 bg-black/40 grid place-items-center">
            <div class="w-5 h-5 border-2 border-white/70 border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        <button
          type="button"
          onClick={onRemove}
          class="absolute top-1 right-1 w-5 h-5 rounded-full bg-black/70 hover:bg-accent-red text-white grid place-items-center opacity-0 group-hover:opacity-100 transition-opacity"
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
    <div class="group flex items-center gap-1.5 bg-ink-700 border border-ink-500 rounded-md px-2 py-1.5 text-xs h-9">
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
        class="text-ink-300 hover:text-accent-red flex-none -mr-1"
        aria-label={`Remove ${att.name}`}
      >
        <X class="w-3 h-3" />
      </button>
    </div>
  );
}
