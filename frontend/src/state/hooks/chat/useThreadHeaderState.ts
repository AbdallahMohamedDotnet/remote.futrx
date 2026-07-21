import { useEffect, useState } from "preact/hooks";

export function useThreadHeaderState(cwd: string | undefined, onCwdCommit: (cwd: string) => void) {
  const [editingCwd, setEditingCwd] = useState(false);
  const [cwdInput, setCwdInput] = useState(cwd ?? "");

  useEffect(() => {
    setCwdInput(cwd ?? "");
  }, [cwd]);

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
    editingCwd,
    setEditingCwd,
    cwdInput,
    setCwdInput,
    commitCwd,
    cancelCwdEdit,
  };
}
