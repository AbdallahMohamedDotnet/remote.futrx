import type { ComponentType } from "preact";
import { useEffect, useRef, useState, useCallback } from "preact/hooks";
import type { ChatMeta, ChatMode, ChatProvider, ReasoningEffort } from "../models/chat";
import type { ContainerApp, ProjectMeta } from "../models/project";
import { BrowserDrawer, type BrowserElementCapture } from "../components/chat/BrowserDrawer";
import { ChatThread } from "../components/chat/ChatThread";
import { projectService } from "../services/projectService";
import { useChat } from "../hooks/chat/useChat";
import { useAttachmentUpload } from "../hooks/chat/useAttachmentUpload";
import { useAutosizeTextarea } from "../hooks/chat/useAutosizeTextarea";
import { useChatMetaActions } from "../hooks/chat/useChatMetaActions";
import { useDragUpload } from "../hooks/chat/useDragUpload";
import { usePromptQueue } from "../hooks/chat/usePromptQueue";
import { useThreadHeaderState } from "../hooks/chat/useThreadHeaderState";
import { useThreadScroll } from "../hooks/chat/useThreadScroll";
import { chatService } from "../services/chatService";
import {
  estimateCost,
  formatTokens,
  modelOptionsForProvider,
  modelDisplayLabel,
  tokenTotal,
} from "../state/chat/usage";
import type { Block } from "../state/chat/messageBlocks";

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
  const [containerApps, setContainerApps] = useState<ContainerApp[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [selectedAppPort, setSelectedAppPort] = useState<number | null>(null);
  const [TerminalOverlay, setTerminalOverlay] = useState<TerminalOverlayComponent | null>(null);
  const readMarkerRef = useRef("");
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
  const browserUrl = browserProject ? latestPublicDevUrl(blocks, browserProject.slug) : "";

  useEffect(() => {
    setText("");
    setTerminalOpen(false);
    setBrowserOpen(false);
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
    if (status !== "ready") return;
    const key = `${chat.id}:${eventCount}`;
    if (readMarkerRef.current === key) return;
    readMarkerRef.current = key;
    void chatService.markRead(chat.id).then(onMetaUpdate).catch(() => {});
  }, [chat.id, eventCount, onMetaUpdate, status]);

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

  const loadContainerApps = useCallback(async () => {
    if (!displayMeta.projectId) {
      setContainerApps([]);
      setSelectedAppPort(null);
      return;
    }
    setAppsLoading(true);
    try {
      const apps = await projectService.listApps(displayMeta.projectId);
      setContainerApps(apps);
      setSelectedAppPort((prev) => {
        if (apps.length === 0) return null;
        // Prefer keeping the current selection if it's still listening.
        if (prev != null && apps.some((app) => app.port === prev)) return prev;
        // Else fall back to a port the agent recently mentioned in chat.
        const hinted = portFromChatUrl(browserUrl);
        if (hinted != null && apps.some((app) => app.port === hinted)) return hinted;
        // Else pick the highest-numbered port — usually the user-bound app
        // rather than a system service (ssh, systemd-resolved, etc.).
        return apps[apps.length - 1].port;
      });
    } catch {
      setContainerApps([]);
      setSelectedAppPort(null);
    } finally {
      setAppsLoading(false);
    }
  }, [displayMeta.projectId, browserUrl]);

  function openBrowserDrawer() {
    if (!displayMeta.projectId) {
      alert("This chat is not attached to a project container.");
      return;
    }
    setBrowserOpen(true);
    void loadContainerApps();
  }

  function insertBrowserElementContext(capture: BrowserElementCapture) {
    const insertion = `\n\n${formatBrowserElementCapture(capture)}\n\n`;
    const textarea = textareaRef.current;
    const start = textarea?.selectionStart ?? text.length;
    const end = textarea?.selectionEnd ?? start;
    const next = `${text.slice(0, start)}${insertion}${text.slice(end)}`;
    setText(next);
    setTimeout(() => {
      textareaRef.current?.focus();
      const pos = start + insertion.length;
      textareaRef.current?.setSelectionRange(pos, pos);
    }, 0);
  }

  return (
    <div class="relative flex-1 h-full min-h-0 overflow-hidden">
      <div class="flex h-full min-h-0 w-full overflow-hidden">
        <div class="min-w-0 flex-1 h-full">
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
          />
        </div>
        <BrowserDrawer
          open={browserOpen}
          projectName={browserProject?.name || ""}
          projectSlug={browserProject?.slug || ""}
          apps={containerApps}
          appsLoading={appsLoading}
          selectedPort={selectedAppPort}
          onSelectPort={setSelectedAppPort}
          onRefreshApps={() => void loadContainerApps()}
          onCaptureElement={insertBrowserElementContext}
          onClose={() => setBrowserOpen(false)}
        />
      </div>
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

function latestPublicDevUrl(blocks: Block[], slug: string): string {
  let latest = "";
  for (const block of blocks) {
    for (const text of blockTexts(block)) {
      for (const candidate of publicDevUrls(text)) {
        if (isProjectDevUrl(candidate, slug)) latest = candidate;
      }
    }
  }
  return latest;
}

function blockTexts(block: Block): string[] {
  if (block.type === "user") return [block.text];
  if (block.type === "error") return [block.message];
  return block.parts.flatMap((part) => {
    if (part.kind === "text" || part.kind === "thinking") return [part.text];
    return part.output ? [part.output] : [];
  });
}

function publicDevUrls(text: string): string[] {
  return [...text.matchAll(/https:\/\/[a-z0-9][a-z0-9-]*--\d{4,5}\.dev\.remote\.futrx\.dev[^\s<>)\]]*/g)]
    .map((match) => match[0].replace(/[.,;:!?]+$/, ""));
}

function isProjectDevUrl(raw: string, slug: string): boolean {
  try {
    const url = new URL(raw);
    const suffix = ".dev.remote.futrx.dev";
    const portStart = `${slug}--`;
    return (
      url.protocol === "https:" &&
      url.hostname.startsWith(portStart) &&
      url.hostname.endsWith(suffix) &&
      validDevPort(url.hostname.slice(portStart.length, -suffix.length))
    );
  } catch {
    return false;
  }
}

function validDevPort(port: string): boolean {
  const value = Number(port);
  return Number.isInteger(value) && value >= 1024 && value <= 65535;
}

function formatBrowserElementCapture(capture: BrowserElementCapture): string {
  const lines = [
    "[Browser element]",
    `URL: ${capture.url || ""}`,
  ];
  if (capture.title) lines.push(`Title: ${capture.title}`);
  lines.push(`Selector: ${capture.selector || ""}`);
  lines.push(`Tag: ${capture.tag || ""}`);
  if (capture.id) lines.push(`ID: ${capture.id}`);
  if (capture.classes?.length) lines.push(`Classes: ${capture.classes.join(" ")}`);
  if (capture.role) lines.push(`Role: ${capture.role}`);
  if (capture.ariaLabel) lines.push(`ARIA label: ${capture.ariaLabel}`);
  if (capture.rect) {
    lines.push(`Box: x=${capture.rect.x} y=${capture.rect.y} w=${capture.rect.width} h=${capture.rect.height}`);
  }
  if (capture.viewport) {
    lines.push(`Viewport: ${capture.viewport.width}x${capture.viewport.height}`);
  }
  if (capture.parents?.length) {
    lines.push(`Parents: ${capture.parents.join(" > ")}`);
  }
  if (capture.styles && Object.keys(capture.styles).length) {
    lines.push("Styles:");
    for (const [key, value] of Object.entries(capture.styles)) {
      if (value) lines.push(`- ${key}: ${value}`);
    }
  }
  if (capture.text) lines.push(`Text: ${capture.text}`);
  if (capture.html) lines.push(`HTML: ${capture.html}`);
  lines.push("[/Browser element]");
  return lines.join("\n");
}

function portFromChatUrl(url: string): number | null {
  const m = /--(\d{4,5})\./.exec(url);
  return m ? Number(m[1]) : null;
}
