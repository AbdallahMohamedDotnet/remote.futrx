import { useEffect, useState } from "preact/hooks";

export function useChatDrawerController({
  chatId,
  showBrowser,
  hideBrowser,
}: {
  chatId: string;
  showBrowser: () => void;
  hideBrowser: () => void;
}) {
  const [historyOpen, setHistoryOpen] = useState(false);
  const [filesOpen, setFilesOpen] = useState(false);
  const [terminalOpen, setTerminalOpen] = useState(false);

  useEffect(() => {
    setHistoryOpen(false);
    setFilesOpen(false);
    setTerminalOpen(false);
  }, [chatId]);

  function openBrowser() {
    setHistoryOpen(false);
    setFilesOpen(false);
    setTerminalOpen(false);
    showBrowser();
  }

  function openHistory() {
    hideBrowser();
    setFilesOpen(false);
    setTerminalOpen(false);
    setHistoryOpen(true);
  }

  function openFiles() {
    hideBrowser();
    setHistoryOpen(false);
    setTerminalOpen(false);
    setFilesOpen(true);
  }


  function openTerminal() {
    hideBrowser();
    setHistoryOpen(false);
    setFilesOpen(false);
    setTerminalOpen(true);
  }

  return {
    historyOpen,
    filesOpen,
    terminalOpen,
    openBrowser,
    openHistory,
    openFiles,
    openTerminal,
    closeHistory: () => setHistoryOpen(false),
    closeFiles: () => setFilesOpen(false),
    closeTerminal: () => setTerminalOpen(false),
  };
}
