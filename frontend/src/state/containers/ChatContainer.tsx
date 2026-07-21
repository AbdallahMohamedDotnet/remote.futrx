import { useEffect, useState } from "preact/hooks";
import type { ChatMeta, ChatMode, ChatProvider, ReasoningEffort, SelectedSkill } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import type { RegisteredSkill } from "../../models/skill";
import { BrowserDrawer } from "../../ui/chat/browser/BrowserDrawer";
import { ChatThread } from "../../ui/chat/ChatThread";
import { HistoryDrawer } from "../../ui/chat/history/HistoryDrawer";
import { FileManagerDrawer } from "../../ui/chat/files/FileManagerDrawer";
import { useChat } from "../../hooks/chat/useChat";
import { useChatBrowserController } from "../../hooks/chat/useChatBrowserController";
import { useChatComposerController } from "../../hooks/chat/useChatComposerController";
import { useChatKeyboardShortcuts } from "../../hooks/chat/useChatKeyboardShortcuts";
import { useChatMetaActions } from "../../hooks/chat/useChatMetaActions";
import { useChatReadMarker } from "../../hooks/chat/useChatReadMarker";
import { useTerminalOverlayController } from "../../hooks/chat/useTerminalOverlayController";
import { useThreadHeaderState } from "../../hooks/chat/useThreadHeaderState";
import { useUserSettingsContext } from "../context/UserSettingsContext";

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
  const { settings, setChatSettings } = useUserSettingsContext();
  const baseMeta = meta ?? chat;
  const displayProvider = baseMeta.provider || settings.chat.provider;
  const displayMode = baseMeta.mode || settings.chat.mode;
  const displayMeta: ChatMeta = {
    ...baseMeta,
    provider: displayProvider,
    model: baseMeta.model ?? settings.chat.model,
    mode: displayMode,
    reasoningEffort: baseMeta.reasoningEffort ?? settings.chat.reasoningEffort,
  };
  const selectedSkills = displayMeta.selectedSkills || [];
  const attachmentBasePath = attachmentBasePathForChat(displayMeta, projects);
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
    attachmentBasePath,
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
  const [historyOpen, setHistoryOpen] = useState(false);
  const [filesOpen, setFilesOpen] = useState(false);

  useEffect(() => {
    terminal.resetTerminal();
    setHistoryOpen(false);
    setFilesOpen(false);
  }, [chat.id]);

  useChatReadMarker({ chatId: chat.id, eventCount, status, onMetaUpdate });
  useChatKeyboardShortcuts({ status, onCancel: cancel });

  function changeProvider(provider: ChatProvider) {
    if (provider === displayProvider) return;
    metaActions.applyMeta({ provider, model: "", reasoningEffort: "", selectedSkills: [] });
    void setChatSettings({ provider, model: "", reasoningEffort: "" });
  }

  function selectedSkillKey(skill: SelectedSkill | RegisteredSkill) {
    const provider = skill.provider || displayProvider;
    const source = skill.source || "";
    const command = (skill.command || skill.name).trim().toLowerCase();
    return `${provider}:${source.toLowerCase()}:${command}`;
  }

  function selectSkill(skill: RegisteredSkill) {
    const next: SelectedSkill = {
      name: skill.name,
      command: skill.command || skill.name,
      provider: skill.provider || displayProvider,
      source: skill.source,
    };
    if (selectedSkills.some((selected) => selectedSkillKey(selected) === selectedSkillKey(next))) return;
    metaActions.applyMeta({ selectedSkills: [...selectedSkills, next] });
  }

  function removeSelectedSkill(skill: SelectedSkill) {
    const key = selectedSkillKey(skill);
    metaActions.applyMeta({ selectedSkills: selectedSkills.filter((selected) => selectedSkillKey(selected) !== key) });
  }

  function changeModel(model: string) {
    metaActions.applyMeta({ model });
    void setChatSettings({ model });
  }

  function changeMode(mode: ChatMode) {
    metaActions.applyMeta({ mode });
    void setChatSettings({ mode });
  }

  function changeReasoningEffort(reasoningEffort: ReasoningEffort) {
    metaActions.applyMeta({ reasoningEffort });
    void setChatSettings({ reasoningEffort });
  }

  function openBrowserDrawer() {
    setHistoryOpen(false);
    setFilesOpen(false);
    browser.openBrowserDrawer();
  }

  function openHistoryDrawer() {
    browser.closeBrowserDrawer();
    setFilesOpen(false);
    setHistoryOpen(true);
  }

  function openFileManager() {
    browser.closeBrowserDrawer();
    setHistoryOpen(false);
    setFilesOpen(true);
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
            header={{
              editingCwd: header.editingCwd,
              cwdInput: header.cwdInput,
              onStartEditCwd: () => header.setEditingCwd(true),
              onCwdInput: header.setCwdInput,
              onCommitCwd: header.commitCwd,
              onCancelCwdEdit: header.cancelCwdEdit,
            }}
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
            onModelChange={changeModel}
            onModeChange={changeMode}
            onReasoningEffortChange={changeReasoningEffort}
            onSelectSkill={selectSkill}
            onRemoveSelectedSkill={removeSelectedSkill}
            onOpenTerminal={terminal.openTerminal}
            onOpenBrowser={openBrowserDrawer}
            onOpenHistory={openHistoryDrawer}
            onOpenFiles={openFileManager}
          />
        </div>
        <HistoryDrawer
          chatId={chat.id}
          open={historyOpen}
          onClose={() => setHistoryOpen(false)}
        />
        <FileManagerDrawer
          chatId={chat.id}
          open={filesOpen}
          onClose={() => setFilesOpen(false)}
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

function attachmentBasePathForChat(chat: ChatMeta, projects: ProjectMeta[]) {
  // Uploads are stored in a fixed .uploads/ directory at the workspace root,
  // matching the server (chat.UploadTarget). Anchoring at the root rather than
  // the chat's cwd keeps the path stable and exactly predictable here, so the
  // prompt path always matches where the server actually wrote the file.
  const project = chat.projectId ? projects.find((item) => item.id === chat.projectId) : undefined;
  if (project) return "/workspace/.uploads";

  const cwd = normalizePath(chat.cwd || "");
  return cwd ? `${cwd}/.uploads` : "/.uploads";
}

function normalizePath(path: string) {
  const trimmed = path.trim();
  if (!trimmed) return "";
  return trimmed.replace(/\/+$/, "") || "/";
}
