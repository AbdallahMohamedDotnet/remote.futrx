import { useEffect, useRef, useState } from "preact/hooks";

export function useThreadHeaderState(cwd: string | undefined, onCwdCommit: (cwd: string) => void) {
  const [modelOpen, setModelOpen] = useState(false);
  const [editingCwd, setEditingCwd] = useState(false);
  const [cwdInput, setCwdInput] = useState(cwd ?? "");
  const modelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setCwdInput(cwd ?? "");
  }, [cwd]);

  useEffect(() => {
    if (!modelOpen) return;
    const handler = (event: MouseEvent) => {
      if (modelRef.current && !modelRef.current.contains(event.target as Node)) {
        setModelOpen(false);
      }
    };
    window.addEventListener("mousedown", handler);
    return () => window.removeEventListener("mousedown", handler);
  }, [modelOpen]);

  function commitCwd() {
    const next = cwdInput.trim();
    setEditingCwd(false);
    if (next !== (cwd ?? "")) onCwdCommit(next);
  }

  function cancelCwdEdit() {
    setCwdInput(cwd ?? "");
    setEditingCwd(false);
  }

  return {
    modelRef,
    modelOpen,
    setModelOpen,
    editingCwd,
    setEditingCwd,
    cwdInput,
    setCwdInput,
    commitCwd,
    cancelCwdEdit,
  };
}
