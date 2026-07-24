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
  const [schedulesOpen, setSchedulesOpen] = useState(false);

  useEffect(() => {
    setHistoryOpen(false);
    setFilesOpen(false);
    setSchedulesOpen(false);
  }, [chatId]);

  function openBrowser() {
    setHistoryOpen(false);
    setFilesOpen(false);
    setSchedulesOpen(false);
    showBrowser();
  }

  function openHistory() {
    hideBrowser();
    setFilesOpen(false);
    setSchedulesOpen(false);
    setHistoryOpen(true);
  }

  function openFiles() {
    hideBrowser();
    setHistoryOpen(false);
    setSchedulesOpen(false);
    setFilesOpen(true);
  }

  function openSchedules() {
    hideBrowser();
    setHistoryOpen(false);
    setFilesOpen(false);
    setSchedulesOpen(true);
  }

  return {
    historyOpen,
    filesOpen,
    schedulesOpen,
    openBrowser,
    openHistory,
    openFiles,
    openSchedules,
    closeHistory: () => setHistoryOpen(false),
    closeFiles: () => setFilesOpen(false),
    closeSchedules: () => setSchedulesOpen(false),
  };
}
