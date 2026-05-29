import type { Attachment } from "../../models/upload";
import { formatBytes } from "../../lib/files";
import { File as FileIcon, X } from "../ui/icons";

export function AttachmentChip({ attachment, onRemove }: { attachment: Attachment; onRemove: () => void }) {
  const pending = !attachment.serverPath;

  if (attachment.isImage && attachment.objectUrl) {
    return (
      <div class="relative w-20 h-20 rounded-lg overflow-hidden bg-[#101318] border border-white/10 group">
        <img src={attachment.objectUrl} class="w-full h-full object-cover" alt={attachment.name} />
        {pending && (
          <div class="absolute inset-0 bg-black/40 grid place-items-center">
            <div class="w-5 h-5 border-2 border-white/70 border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        <button
          type="button"
          onClick={onRemove}
          class="absolute top-1 right-1 w-6 h-6 rounded-md bg-black/70 hover:bg-accent-red text-white grid place-items-center opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
          aria-label={`Remove ${attachment.name}`}
        >
          <X class="w-3 h-3" />
        </button>
        <div class="absolute bottom-0 left-0 right-0 px-1.5 py-0.5 bg-gradient-to-t from-black/85 to-transparent text-white text-[9.5px] truncate">
          {attachment.name}
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
      <span class="truncate max-w-[180px] text-ink-100" title={attachment.name}>{attachment.name}</span>
      <span class="text-ink-300 text-[10px] flex-none">{formatBytes(attachment.size)}</span>
      <button
        type="button"
        onClick={onRemove}
        class="w-6 h-6 grid place-items-center rounded text-ink-300 hover:text-accent-red hover:bg-accent-red/10 flex-none -mr-1"
        aria-label={`Remove ${attachment.name}`}
      >
        <X class="w-3 h-3" />
      </button>
    </div>
  );
}
