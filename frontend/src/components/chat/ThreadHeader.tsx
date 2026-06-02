import type { RefObject } from "preact";
import type { ChatMeta } from "../../models/chat";
import type { UsageTotals } from "../../state/chat/usage";
import { providerDisplayLabel } from "../../state/chat/usage";
import { Menu, MessageSquare } from "../ui/icons";
import { CwdEditor } from "./CwdEditor";
import { ModelPicker } from "./ModelPicker";
import { UsagePill } from "./UsagePill";

export function ThreadHeader({
  chat,
  streaming,
  modelRef,
  modelOpen,
  modelOptions,
  modelDisplayLabel,
  editingCwd,
  cwdInput,
  usageTotals,
  tokenLabel,
  costUsd,
  onToggleModel,
  onPickModel,
  onStartEditCwd,
  onCwdInput,
  onCommitCwd,
  onCancelCwdEdit,
  onOpenTerminal,
  onOpenBrowser,
  onHamburger,
}: {
  chat: ChatMeta;
  streaming: boolean;
  modelRef: RefObject<HTMLDivElement>;
  modelOpen: boolean;
  modelOptions: Array<{ value: string; label: string; sub: string }>;
  modelDisplayLabel: (model?: string) => string;
  editingCwd: boolean;
  cwdInput: string;
  usageTotals: UsageTotals;
  tokenLabel: string;
  costUsd: number;
  onToggleModel: () => void;
  onPickModel: (model: string) => void;
  onStartEditCwd: () => void;
  onCwdInput: (value: string) => void;
  onCommitCwd: () => void;
  onCancelCwdEdit: () => void;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onHamburger: () => void;
}) {
  return (
    <header class="codex-header top-chrome flex-none z-20 bg-[#101318] md:bg-[#101318]/95 md:backdrop-blur border-b border-white/10 px-3 md:px-4 pb-2 flex flex-col gap-2">
      <div class="flex items-center gap-2 min-h-11">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 rounded-md text-ink-100 hover:bg-white/[0.08] grid place-items-center flex-none"
          aria-label="Open chats"
          title="Chats"
        >
          <Menu class="w-5 h-5" />
        </button>

        <div class="hidden sm:grid h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 text-ink-200 place-items-center flex-none">
          <MessageSquare class="w-4 h-4" />
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 min-w-0">
            <h1 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
              {chat.title || "Untitled chat"}
            </h1>
            <span
              class={`h-2 w-2 rounded-full flex-none ${streaming ? "bg-accent-green animate-pulse" : "bg-ink-400"}`}
              title={streaming ? "Streaming" : "Ready"}
            />
          </div>
          <div class="text-[12px] text-ink-300 truncate">
            {providerDisplayLabel(chat.provider)} · {streaming ? "Working" : "Ready"}
          </div>
        </div>

        <ModelPicker
          modelRef={modelRef}
          open={modelOpen}
          model={chat.model}
          streaming={streaming}
          options={modelOptions}
          displayLabel={modelDisplayLabel}
          onToggle={onToggleModel}
          onPick={onPickModel}
        />
      </div>

      <div class="flex items-center gap-2 overflow-x-auto no-scrollbar pb-0.5">
        <CwdEditor
          editing={editingCwd}
          cwd={chat.cwd || "~"}
          value={cwdInput}
          onStartEdit={onStartEditCwd}
          onChange={onCwdInput}
          onCommit={onCommitCwd}
          onCancel={onCancelCwdEdit}
          onOpenTerminal={onOpenTerminal}
          onOpenBrowser={onOpenBrowser}
        />
        <UsagePill totals={usageTotals} tokenLabel={tokenLabel} costUsd={costUsd} />
      </div>
    </header>
  );
}
