import type { ComponentType } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import type { ChatMeta, ChatMode, ChatProvider, ReasoningEffort } from "../models/chat";
import type { ProjectMeta } from "../models/project";
import { BrowserDrawer } from "../components/chat/BrowserDrawer";
import { ChatThread } from "../components/chat/ChatThread";
import { useChat } from "../hooks/chat/useChat";
import { useAttachmentUpload } from "../hooks/chat/useAttachmentUpload";
import { useAutosizeTextarea } from "../hooks/chat/useAutosizeTextarea";
import { useChatMetaActions } from "../hooks/chat/useChatMetaActions";
import { useDragUpload } from "../hooks/chat/useDragUpload";
import { usePromptQueue } from "../hooks/chat/usePromptQueue";
import { useThreadHeaderState } from "../hooks/chat/useThreadHeaderState";
import { useThreadScroll } from "../hooks/chat/useThreadScroll";
import { projectService } from "../services/projectService";
import {
  estimateCost,
  formatTokens,
  modelOptionsForProvider,
  modelDisplayLabel,
  tokenTotal,
} from "../state/chat/usage";

type TerminalOverlayComponent = ComponentType<{
  chat: ChatMeta;
  open: boolean;
  onClose: () => void;
}>;

export function ChatContainer({
  chat,
  projects,
  onHamburger,
  onMetaUpdate,
}: {
  chat: ChatMeta;
  projects: ProjectMeta[];
  onHamburger: () => void;
  onMetaUpdate: () => void;
}) {
  const {
    meta,
    blocks,
    usageTotals,
    eventCount,
    hasOlder,
    loadingOlder,
    status,
    error,
    canSendPrompt,
    sendPrompt,
    cancel,
    rewind,
    loadOlder,
    refreshMeta,
  } = useChat(chat.id);
  const displayMeta = meta ?? chat;
  const displayProvider = displayMeta.provider || "claude";
  const displayMode = displayMeta.mode || "code";
  const [text, setText] = useState("");
  const [terminalOpen, setTerminalOpen] = useState(false);
  const [browserOpen, setBrowserOpen] = useState(false);
  const [browserPort, setBrowserPort] = useState(() => loadBrowserPort(chat.projectId || ""));
  const [openingDatabase, setOpeningDatabase] = useState(false);
  const [TerminalOverlay, setTerminalOverlay] = useState<TerminalOverlayComponent | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { textareaRef, focusInput } = useAutosizeTextarea(text);
  const upload = useAttachmentUpload(chat.id, onMetaUpdate);
  const drag = useDragUpload(upload.doUpload);
  const scroll = useThreadScroll(chat.id, `${eventCount}:${blocks.length}`);
  const metaActions = useChatMetaActions({
    chatId: chat.id,
    refreshMeta,
    onMetaUpdate,
  });
  const header = useThreadHeaderState(displayMeta.cwd, (cwd) => metaActions.applyMeta({ cwd }));
  const queue = usePromptQueue({
    chatId: chat.id,
    status,
    canSendPrompt,
    sendPrompt,
    onSent: scroll.unlockAutoScroll,
  });
  const costUsd = displayProvider === "claude" ? estimateCost(usageTotals, displayMeta.model || "") : 0;
  const tokenLabel = formatTokens(tokenTotal(usageTotals));
  const browserProject = displayMeta.projectId
    ? projects.find((project) => project.id === displayMeta.projectId) ?? null
    : null;
  const browserUrl = browserProject && validBrowserPort(browserPort)
    ? `https://${browserProject.slug}--${browserPort}.dev.remote.futrx.dev`
    : "";

  useEffect(() => {
    setText("");
    setTerminalOpen(false);
    setBrowserOpen(false);
    setOpeningDatabase(false);
    setBrowserPort(loadBrowserPort(chat.projectId || ""));
    scroll.unlockAutoScroll();
  }, [chat.id]);

  useEffect(() => {
    if (!terminalOpen || TerminalOverlay) return;
    let cancelled = false;
    import("../components/chat/TerminalOverlay").then((module) => {
      if (!cancelled) setTerminalOverlay(() => module.TerminalOverlay);
    });
    return () => {
      cancelled = true;
    };
  }, [TerminalOverlay, terminalOpen]);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape" && status === "streaming") cancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [status, cancel]);

  async function handleRewind(t: number, promptText: string) {
    if (status === "streaming") {
      alert("Cancel the current run before rewinding this chat.");
      return;
    }
    if (!confirm("Rewind to this prompt? Messages from this point forward will be removed.")) return;
    try {
      await rewind(t);
      queue.clearQueuedPrompts();
      setText(promptText);
      await refreshMeta();
      onMetaUpdate();
      scroll.unlockAutoScroll();
      setTimeout(() => {
        scroll.jumpToBottom();
        focusInput();
      }, 0);
    } catch (rewindError) {
      alert("rewind failed: " + (rewindError as Error).message);
    }
  }

  function handlePaste(event: ClipboardEvent) {
    const items = event.clipboardData?.items;
    if (!items) return;
    const files: File[] = [];
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (item.kind === "file") {
        const file = item.getAsFile();
        if (file) files.push(file);
      }
    }
    if (files.length) {
      event.preventDefault();
      upload.doUpload(files);
    }
  }

  function handleSend() {
    if (upload.uploading || (!statusAllowsQueue(status) && !canSendPrompt)) return;
    const userText = text.trim();
    const paths = upload.attachments
      .filter((attachment) => attachment.serverPath)
      .map((attachment) => attachment.serverPath);
    if (!userText && paths.length === 0) return;
    const finalText = paths.length
      ? (userText ? `${userText}\n\n${paths.join(" ")}` : paths.join(" "))
      : userText;

    if (status === "streaming") {
      queue.queuePrompt(finalText);
    } else {
      const sent = sendPrompt(finalText);
      if (!sent) return;
    }

    setText("");
    upload.clearAttachments();
    scroll.unlockAutoScroll();
    setTimeout(focusInput, 0);
  }

  function pickModel(model: string) {
    header.setModelOpen(false);
    if (model !== displayMeta.model) metaActions.applyMeta({ model });
  }

  function changeProvider(provider: ChatProvider) {
    if (provider === displayProvider) return;
    metaActions.applyMeta({ provider, model: "", reasoningEffort: "" });
  }

  function changeMode(mode: ChatMode) {
    metaActions.applyMeta({ mode });
  }

  function changeReasoningEffort(reasoningEffort: ReasoningEffort) {
    metaActions.applyMeta({ reasoningEffort });
  }

  function openBrowserDrawer() {
    if (!displayMeta.projectId) {
      alert("This chat is not attached to a project container.");
      return;
    }
    setBrowserOpen(true);
  }

  function changeBrowserPort(value: string) {
    const port = value.replace(/\D/g, "").slice(0, 5);
    setBrowserPort(port);
    if (displayMeta.projectId && validBrowserPort(port)) {
      saveBrowserPort(displayMeta.projectId, port);
    }
  }

  async function openDatabaseViewer() {
    if (!displayMeta.projectId) {
      alert("This chat is not attached to a project container.");
      return;
    }
    if (openingDatabase) return;

    const popup = window.open("", "_blank");
    setOpeningDatabase(true);
    try {
      const viewer = await projectService.openDBViewer(displayMeta.projectId);
      if (popup) {
        popup.opener = null;
        popup.location.href = viewer.url;
      } else {
        window.open(viewer.url, "_blank", "noopener,noreferrer");
      }
    } catch (dbError) {
      if (popup) popup.close();
      alert("open db viewer failed: " + (dbError as Error).message);
    } finally {
      setOpeningDatabase(false);
    }
  }

  return (
    <div class="relative flex-1 h-full min-h-0 overflow-hidden">
      <ChatThread
        chat={displayMeta}
        blocks={blocks}
        hasOlder={hasOlder}
        loadingOlder={loadingOlder}
        status={status}
        error={error}
        canSendPrompt={canSendPrompt}
        streaming={status === "streaming"}
        mode={displayMode}
        queuedPrompts={queue.queuedPrompts}
        attachments={upload.attachments}
        uploading={upload.uploading}
        dragging={drag.dragging}
        text={text}
        textareaRef={textareaRef}
        fileInputRef={fileInputRef}
        showJump={scroll.showJump}
        scrollRef={scroll.scrollRef}
        contentRef={scroll.contentRef}
        bottomRef={scroll.bottomRef}
        header={{
          modelRef: header.modelRef,
          modelOpen: header.modelOpen,
          modelOptions: modelOptionsForProvider(displayProvider),
          modelDisplayLabel: (model) => modelDisplayLabel(model, displayProvider),
          editingCwd: header.editingCwd,
          cwdInput: header.cwdInput,
          onToggleModel: () => header.setModelOpen(!header.modelOpen),
          onPickModel: pickModel,
          onStartEditCwd: () => header.setEditingCwd(true),
          onCwdInput: header.setCwdInput,
          onCommitCwd: header.commitCwd,
          onCancelCwdEdit: header.cancelCwdEdit,
        }}
        usageTotals={usageTotals}
        tokenLabel={tokenLabel}
        costUsd={costUsd}
        onHamburger={onHamburger}
        onScroll={scroll.onScroll}
        onJumpToBottom={scroll.jumpToBottom}
        onAnswerQuestion={(answer) => {
          const sent = sendPrompt(answer);
          if (sent) scroll.unlockAutoScroll();
        }}
        onLoadOlder={loadOlder}
        onRewind={handleRewind}
        onTextChange={setText}
        onFilesSelected={upload.doUpload}
        onPaste={handlePaste}
        onSend={handleSend}
        onCancel={cancel}
        onRemoveQueued={queue.removeQueuedPrompt}
        onRemoveAttachment={upload.removeAttachment}
        onProviderChange={changeProvider}
        onModelChange={(model) => metaActions.applyMeta({ model })}
        onModeChange={changeMode}
        onReasoningEffortChange={changeReasoningEffort}
        onOpenTerminal={() => setTerminalOpen(true)}
        onOpenBrowser={openBrowserDrawer}
        onOpenDatabase={openDatabaseViewer}
        openingDatabase={openingDatabase}
      />
      <BrowserDrawer
        open={browserOpen}
        projectName={browserProject?.name || ""}
        url={browserUrl}
        port={browserPort}
        onPortChange={changeBrowserPort}
        onClose={() => setBrowserOpen(false)}
      />
      {TerminalOverlay && (
        <TerminalOverlay
          chat={displayMeta}
          open={terminalOpen}
          onClose={() => setTerminalOpen(false)}
        />
      )}
    </div>
  );
}

function statusAllowsQueue(status: string): boolean {
  return status === "streaming";
}

function browserPortKey(projectId: string): string {
  return `remote-browser-port:${projectId}`;
}

function loadBrowserPort(projectId: string): string {
  if (!projectId) return "3000";
  try {
    return localStorage.getItem(browserPortKey(projectId)) || "3000";
  } catch {
    return "3000";
  }
}

function saveBrowserPort(projectId: string, port: string) {
  try {
    localStorage.setItem(browserPortKey(projectId), port);
  } catch {}
}

function validBrowserPort(port: string): boolean {
  const value = Number(port);
  return Number.isInteger(value) && value >= 1024 && value <= 65535;
}
