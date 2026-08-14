import type { RefObject } from "preact";
import { Plus } from "../../primitives/icons";

export function AttachButton({
  fileInputRef,
  uploading,
  disconnected,
  onFilesSelected,
}: {
  fileInputRef: RefObject<HTMLInputElement>;
  uploading: boolean;
  disconnected: boolean;
  onFilesSelected: (files: File[]) => void;
}) {
  return (
    <>
      <button
        type="button"
        onClick={() => fileInputRef.current?.click()}
        disabled={uploading || disconnected}
        class="codex-icon-button flex-none w-10 h-10 rounded-lg bg-white/[0.045]
               hover:bg-white/10 active:bg-accent-blue active:border-accent-blue active:scale-[0.98]
               disabled:opacity-50 disabled:active:scale-100 grid place-items-center text-ink-300 hover:text-ink-100 transition"
        aria-label="Attach files"
        title="Attach (or drag-and-drop / paste images)"
      >
        <Plus class="w-4 h-4" />
      </button>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        class="hidden"
        onChange={(event) => {
          const input = event.currentTarget as HTMLInputElement;
          const files = Array.from(input.files || []);
          input.value = "";
          onFilesSelected(files);
        }}
      />
    </>
  );
}
