import { useCallback, useEffect, useState } from "preact/hooks";
import type { Attachment } from "../../models/upload";
import { uploadChatFiles } from "../../services/uploadService";
import { randomId } from "../../lib/ids";

export function useAttachmentUpload(chatId: string, onAfterUpload?: () => void) {
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    clearAttachments();
  }, [chatId]);

  useEffect(() => () => clearAttachments(), []);

  const doUpload = useCallback(async (files: File[]) => {
    if (!files.length) return;

    const localized: Attachment[] = files.map((file) => ({
      id: randomId(),
      name: file.name,
      size: file.size,
      serverPath: "",
      isImage: file.type.startsWith("image/"),
      objectUrl: file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined,
    }));

    setAttachments((prev) => [...prev, ...localized]);
    setUploading(true);
    try {
      const response = await uploadChatFiles(chatId, files);
      setAttachments((prev) => {
        const next = [...prev];
        for (let index = 0; index < response.results.length; index++) {
          const result = response.results[index];
          const localIndex = next.findIndex((attachment) => attachment.id === localized[index].id);
          if (localIndex < 0) continue;
          if (result.error) {
            revokeAttachment(next[localIndex]);
            next.splice(localIndex, 1);
          } else {
            next[localIndex] = { ...next[localIndex], serverPath: result.path || "" };
          }
        }
        return next;
      });

      const failed = response.results.filter((result) => result.error);
      if (failed.length) {
        alert("Failed:\n" + failed.map((result) => `${result.name} - ${result.error}`).join("\n"));
      }
      onAfterUpload?.();
    } catch (error) {
      setAttachments((prev) => {
        const ids = new Set(localized.map((attachment) => attachment.id));
        prev.forEach((attachment) => {
          if (ids.has(attachment.id)) revokeAttachment(attachment);
        });
        return prev.filter((attachment) => !ids.has(attachment.id));
      });
      alert("upload failed: " + (error as Error).message);
    } finally {
      setUploading(false);
    }
  }, [chatId, onAfterUpload]);

  function removeAttachment(id: string) {
    setAttachments((prev) => {
      const target = prev.find((attachment) => attachment.id === id);
      if (target) revokeAttachment(target);
      return prev.filter((attachment) => attachment.id !== id);
    });
  }

  function clearAttachments() {
    setAttachments((prev) => {
      prev.forEach(revokeAttachment);
      return [];
    });
  }

  return {
    attachments,
    uploading,
    doUpload,
    removeAttachment,
    clearAttachments,
  };
}

function revokeAttachment(attachment: Attachment) {
  if (attachment.objectUrl) URL.revokeObjectURL(attachment.objectUrl);
}
