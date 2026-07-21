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

  useEffect(() => {
    setHistoryOpen(false);
    setFilesOpen(false);
  }, [chatId]);

  function openBrowser() {
    setHistoryOpen(false);
    setFilesOpen(false);
    showBrowser();
  }

  function openHistory() {
    hideBrowser();
    setFilesOpen(false);
    setHistoryOpen(true);
  }

  function openFiles() {
    hideBrowser();
    setHistoryOpen(false);
    setFilesOpen(true);
  }

  return {
    historyOpen,
    filesOpen,
    openBrowser,
    openHistory,
    openFiles,
    closeHistory: () => setHistoryOpen(false),
    closeFiles: () => setFilesOpen(false),
  };
}
