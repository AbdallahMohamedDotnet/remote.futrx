import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import type { Attachment } from "../../models/upload";
import { startChatUpload, type UploadHandle } from "../../services/uploadService";
import { randomId } from "../../lib/ids";

export function useAttachmentUpload(chatId: string, onAfterUpload?: () => void) {
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [uploading, setUploading] = useState(false);

  // Outstanding tus handles, keyed by attachment id. Lets us abort on remove.
  const handlesRef = useRef<Map<string, UploadHandle>>(new Map());

  useEffect(() => {
    clearAttachments();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatId]);

  useEffect(
    () => () => {
      clearAttachments();
    },
    []
  );

  const doUpload = useCallback(
    async (files: File[]) => {
      if (!files.length) return;
      const queued: Attachment[] = files.map((file) => ({
        id: randomId(),
        name: file.name,
        size: file.size,
        serverPath: "",
        isImage: file.type.startsWith("image/"),
        objectUrl: file.type.startsWith("image/")
          ? URL.createObjectURL(file)
          : undefined,
        progress: 0,
      }));

      setAttachments((prev) => [...prev, ...queued]);
      setUploading(true);

      const finishedFlags: Promise<void>[] = [];
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        const att = queued[i];
        const done = new Promise<void>((resolve) => {
          const handle = startChatUpload(chatId, file, {
            onProgress(loaded, total) {
              const ratio = total > 0 ? loaded / total : 0;
              setAttachments((prev) =>
                prev.map((a) => (a.id === att.id ? { ...a, progress: ratio } : a))
              );
            },
            onSuccess() {
              handlesRef.current.delete(att.id);
              setAttachments((prev) =>
                prev.map((a) =>
                  a.id === att.id
                    ? { ...a, progress: 1, serverPath: file.name, error: undefined }
                    : a
                )
              );
              resolve();
            },
            onError(err) {
              handlesRef.current.delete(att.id);
              setAttachments((prev) =>
                prev.map((a) =>
                  a.id === att.id ? { ...a, error: err.message } : a
                )
              );
              resolve();
            },
          });
          handlesRef.current.set(att.id, handle);
        });
        finishedFlags.push(done);
      }

      await Promise.all(finishedFlags);
      setUploading(false);
      onAfterUpload?.();
    },
    [chatId, onAfterUpload]
  );

  function removeAttachment(id: string) {
    const handle = handlesRef.current.get(id);
    if (handle) {
      void handle.abort();
      handlesRef.current.delete(id);
    }
    setAttachments((prev) => {
      const target = prev.find((attachment) => attachment.id === id);
      if (target) revokeAttachment(target);
      return prev.filter((attachment) => attachment.id !== id);
    });
  }

  function clearAttachments() {
    for (const handle of handlesRef.current.values()) void handle.abort();
    handlesRef.current.clear();
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
