import type { AgentAuthAccountsSnapshot } from "../../../models/auth";
import { Users } from "../../primitives/icons";

export function ComposerAccountPicker({
  accountId,
  accounts,
  streaming,
  onChange,
}: {
  accountId: string;
  accounts: AgentAuthAccountsSnapshot;
  streaming: boolean;
  onChange: (accountId: string) => void;
}) {
  const selectedId = accountId || accounts.activeAccountId || accounts.items[0]?.id || "";
  const selected = accounts.items.find((account) => account.id === selectedId);

  return (
    <label
      class="relative flex h-7 max-w-[180px] min-w-[116px] items-center gap-1.5 rounded-control px-2 text-ink-300 transition hover:bg-tint-strong hover:text-ink-100"
      title={streaming ? "Cannot change account while streaming" : "Choose account for this chat"}
    >
      <Users class="h-3.5 w-3.5 flex-none opacity-60" aria-hidden="true" />
      <span class="sr-only">Account</span>
      <span class="min-w-0 flex-1 truncate text-[11.5px] font-medium">
        {selected?.label || "Choose account"}
      </span>
      <select
        value={selectedId}
        disabled={streaming}
        onChange={(event) => onChange(event.currentTarget.value)}
        class="absolute inset-0 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
        aria-label="Account"
      >
        {accounts.items.map((account) => (
          <option key={account.id} value={account.id}>
            {account.label}{account.email ? ` (${account.email})` : ""}
          </option>
        ))}
      </select>
    </label>
  );
}
