import type { Attachment } from "../../../models/upload";
import { AttachmentChip } from "./AttachmentChip";

export function AttachmentTray({
  attachments,
  onRemove,
}: {
  attachments: Attachment[];
  onRemove: (id: string) => void;
}) {
  if (attachments.length === 0) return null;

  return (
    <div class="px-3 pb-1 flex flex-wrap gap-2 max-h-[180px] overflow-y-auto touch-scroll scrollbar-thin">
      {attachments.map((attachment) => (
        <AttachmentChip
          key={attachment.id}
          attachment={attachment}
          onRemove={() => onRemove(attachment.id)}
        />
      ))}
    </div>
  );
}
