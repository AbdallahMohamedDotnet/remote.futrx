import { useEffect } from "preact/hooks";
import type { ChatMeta, ChatMode, ChatProvider, ReasoningEffort } from "../models/chat";
import type { ProjectMeta } from "../models/project";
import { BrowserDrawer } from "../components/chat/browser/BrowserDrawer";
import { ChatThread } from "../components/chat/ChatThread";
import { useChat } from "../hooks/chat/useChat";
import { useChatBrowserController } from "../hooks/chat/useChatBrowserController";
import { useChatComposerController } from "../hooks/chat/useChatComposerController";
import { useChatKeyboardShortcuts } from "../hooks/chat/useChatKeyboardShortcuts";
import { useChatMetaActions } from "../hooks/chat/useChatMetaActions";
import { useChatReadMarker } from "../hooks/chat/useChatReadMarker";
import { useTerminalOverlayController } from "../hooks/chat/useTerminalOverlayController";
import { useThreadHeaderState } from "../hooks/chat/useThreadHeaderState";
import {
  estimateCost,
  formatTokens,
  modelOptionsForProvider,
  modelDisplayLabel,
  tokenTotal,
} from "../state/chat/usage";

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
  const metaActions = useChatMetaActions({
    chatId: chat.id,
    refreshMeta,
    onMetaUpdate,
  });
  const header = useThreadHeaderState(displayMeta.cwd, (cwd) => metaActions.applyMeta({ cwd }));
  const composer = useChatComposerController({
    chatId: chat.id,
    eventCount,
    blockCount: blocks.length,
    status,
    canSendPrompt,
    sendPrompt,
    rewind,
    refreshMeta,
    onMetaUpdate,
  });
  const browser = useChatBrowserController({
    chat: displayMeta,
    projects,
    blocks,
    text: composer.text,
    setText: composer.setText,
    textareaRef: composer.textareaRef,
  });
  const terminal = useTerminalOverlayController();
  const costUsd = displayProvider === "claude" ? estimateCost(usageTotals, displayMeta.model || "") : 0;
  const tokenLabel = formatTokens(tokenTotal(usageTotals));

  useEffect(() => {
    terminal.resetTerminal();
  }, [chat.id]);

  useChatReadMarker({ chatId: chat.id, eventCount, status, onMetaUpdate });
  useChatKeyboardShortcuts({ status, onCancel: cancel });

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
            queuedPrompts={composer.queue.queuedPrompts}
            attachments={composer.upload.attachments}
            uploading={composer.upload.uploading}
            dragging={composer.drag.dragging}
            text={composer.text}
            textareaRef={composer.textareaRef}
            fileInputRef={composer.fileInputRef}
            showJump={composer.scroll.showJump}
            scrollRef={composer.scroll.scrollRef}
            contentRef={composer.scroll.contentRef}
            bottomRef={composer.scroll.bottomRef}
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
            onScroll={composer.scroll.onScroll}
            onJumpToBottom={composer.scroll.jumpToBottom}
            onAnswerQuestion={(answer) => {
              const sent = sendPrompt(answer);
              if (sent) composer.scroll.unlockAutoScroll();
            }}
            onLoadOlder={loadOlder}
            onRewind={composer.handleRewind}
            onTextChange={composer.setText}
            onFilesSelected={composer.upload.doUpload}
            onPaste={composer.handlePaste}
            onSend={composer.handleSend}
            onCancel={cancel}
            onRemoveQueued={composer.queue.removeQueuedPrompt}
            onRemoveAttachment={composer.upload.removeAttachment}
            onProviderChange={changeProvider}
            onModelChange={(model) => metaActions.applyMeta({ model })}
            onModeChange={changeMode}
            onReasoningEffortChange={changeReasoningEffort}
            onOpenTerminal={terminal.openTerminal}
            onOpenBrowser={browser.openBrowserDrawer}
          />
        </div>
        <BrowserDrawer
          open={browser.browserOpen}
          projectName={browser.browserProject?.name || ""}
          projectSlug={browser.browserProject?.slug || ""}
          apps={browser.containerApps}
          appsLoading={browser.appsLoading}
          selectedPort={browser.selectedAppPort}
          onSelectPort={browser.setSelectedAppPort}
          onRefreshApps={() => void browser.loadContainerApps()}
          onCaptureElement={browser.insertBrowserElementContext}
          onClose={browser.closeBrowserDrawer}
        />
      </div>
      {terminal.TerminalOverlay && (
        <terminal.TerminalOverlay
          chat={displayMeta}
          open={terminal.terminalOpen}
          onClose={terminal.closeTerminal}
        />
      )}
    </div>
  );
}
