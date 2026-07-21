import { useEffect, useRef, useState } from "preact/hooks";

export function useDragUpload(onFiles: (files: File[]) => void) {
  const [dragging, setDragging] = useState(false);
  const dragCounter = useRef(0);

  useEffect(() => {
    function onDragEnter(event: DragEvent) {
      if (!hasFiles(event)) return;
      event.preventDefault();
      dragCounter.current++;
      setDragging(true);
    }

    function onDragOver(event: DragEvent) {
      if (!hasFiles(event)) return;
      event.preventDefault();
      event.dataTransfer!.dropEffect = "copy";
    }

    function onDragLeave(event: DragEvent) {
      if (!hasFiles(event)) return;
      dragCounter.current = Math.max(0, dragCounter.current - 1);
      if (dragCounter.current === 0) setDragging(false);
    }

    function onDrop(event: DragEvent) {
      if (!event.dataTransfer) return;
      const files = Array.from(event.dataTransfer.files);
      if (!files.length) return;
      event.preventDefault();
      dragCounter.current = 0;
      setDragging(false);
      onFiles(files);
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
  }, [onFiles]);

  return { dragging };
}

function hasFiles(event: DragEvent): boolean {
  return !!event.dataTransfer && Array.from(event.dataTransfer.types).includes("Files");
}
