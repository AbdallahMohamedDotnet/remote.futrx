import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import { BrowserDrawer } from "../../ui/chat/browser/BrowserDrawer";
import { ChatThread } from "../../ui/chat/ChatThread";
import { HistoryDrawer } from "../../ui/chat/history/HistoryDrawer";
import { FileManagerDrawer } from "../../ui/chat/files/FileManagerDrawer";
import { attachmentBasePathForChat } from "../../state/chat/attachmentPaths";
import { useChat } from "../../state/hooks/chat/useChat";
import { useChatBrowserController } from "../../state/hooks/chat/useChatBrowserController";
import { useChatComposerController } from "../../state/hooks/chat/useChatComposerController";
import { useChatDrawerController } from "../../state/hooks/chat/useChatDrawerController";
import { useChatKeyboardShortcuts } from "../../state/hooks/chat/useChatKeyboardShortcuts";
import { useChatPreferences } from "../../state/hooks/chat/useChatPreferences";
import { useChatReadMarker } from "../../state/hooks/chat/useChatReadMarker";
import { useTerminalOverlayController } from "../../state/hooks/chat/useTerminalOverlayController";

export function ChatContainer({
  chat,
  projects,
  onHamburger,
}: {
  chat: ChatMeta;
  projects: ProjectMeta[];
  onHamburger: () => void;
}) {
  const {
    meta,
    blocks,
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
  const preferences = useChatPreferences({ chat, loadedMeta: meta, refreshMeta });
  const { displayMeta, displayMode, selectedSkills } = preferences;
  const attachmentBasePath = attachmentBasePathForChat(displayMeta, projects);
  const composer = useChatComposerController({
    chatId: chat.id,
    eventCount,
    blockCount: blocks.length,
    status,
    canSendPrompt,
    sendPrompt,
    rewind,
    refreshMeta,
    attachmentBasePath,
  });
  const browser = useChatBrowserController({
    chat: displayMeta,
    projects,
    blocks,
    text: composer.text,
    setText: composer.setText,
    textareaRef: composer.textareaRef,
  });
  const terminal = useTerminalOverlayController(chat.id);
  const drawers = useChatDrawerController({
    chatId: chat.id,
    showBrowser: browser.openBrowserDrawer,
    hideBrowser: browser.closeBrowserDrawer,
  });

  useChatReadMarker({ chatId: chat.id, eventCount, status });
  useChatKeyboardShortcuts({ status, onCancel: cancel });

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
            selectedSkills={selectedSkills}
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
            onHamburger={onHamburger}
            onScroll={composer.scroll.onScroll}
            onJumpToBottom={composer.scroll.jumpToBottom}
            onAnswerQuestion={composer.handleAnswerQuestion}
            onLoadOlder={loadOlder}
            onRewind={composer.handleRewind}
            onTextChange={composer.setText}
            onFilesSelected={composer.upload.doUpload}
            onPaste={composer.handlePaste}
            onSend={composer.handleSend}
            onCancel={cancel}
            onRemoveQueued={composer.queue.removeQueuedPrompt}
            onRemoveAttachment={composer.upload.removeAttachment}
            onProviderChange={preferences.changeProvider}
            onModelChange={preferences.changeModel}
            onModeChange={preferences.changeMode}
            onReasoningEffortChange={preferences.changeReasoningEffort}
            onSelectSkill={preferences.selectSkill}
            onRemoveSelectedSkill={preferences.removeSelectedSkill}
            onOpenTerminal={terminal.openTerminal}
            onOpenBrowser={drawers.openBrowser}
            onOpenHistory={drawers.openHistory}
            onOpenFiles={drawers.openFiles}
          />
        </div>
        <HistoryDrawer
          chatId={chat.id}
          open={drawers.historyOpen}
          onClose={drawers.closeHistory}
        />
        <FileManagerDrawer
          chatId={chat.id}
          open={drawers.filesOpen}
          onClose={drawers.closeFiles}
        />
        <BrowserDrawer
          open={browser.browserOpen}
          projectId={browser.browserProject?.id || ""}
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
