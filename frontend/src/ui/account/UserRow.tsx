import { useState } from "preact/hooks";
import type { User, UserRole } from "../../models/user";
import { Check, X } from "../primitives/icons";

export function UserRow({
  user,
  isSelf,
  onRemove,
  onSetRole,
}: {
  user: User;
  isSelf: boolean;
  onRemove: () => Promise<void>;
  onSetRole: (role: UserRole) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const toggleRole = async () => {
    const next: UserRole = user.role === "admin" ? "member" : "admin";
    setBusy(true);
    setErr(null);
    try {
      await onSetRole(next);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    setBusy(true);
    setErr(null);
    try {
      await onRemove();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="rounded-md border border-line bg-tint px-3 py-2 space-y-1">
      <div class="flex items-center gap-2 min-w-0">
        <span class="text-[12.5px] text-ink-50 truncate" title={user.email}>
          {user.email}
        </span>
        <span
          class={`inline-flex items-center h-5 px-1.5 rounded text-[11px] font-medium ${
            user.role === "admin"
              ? "text-accent-blue bg-accent-blue/[0.14]"
              : "text-ink-300 bg-tint"
          }`}
        >
          {user.role}
        </span>
        {isSelf && (
          <span class="inline-flex items-center h-5 px-1.5 rounded text-[11px] text-accent-green bg-accent-green/[0.10]">
            <Check class="w-3 h-3 mr-1" /> you
          </span>
        )}
        <div class="ml-auto flex items-center gap-1">
          <button
            type="button"
            onClick={toggleRole}
            disabled={busy}
            class="h-7 px-2 rounded text-[11px] text-ink-300 hover:text-ink-100 hover:bg-tint-strong disabled:opacity-50"
            title={user.role === "admin" ? "Demote to member" : "Promote to admin"}
          >
            {user.role === "admin" ? "demote" : "promote"}
          </button>
          <button
            type="button"
            onClick={remove}
            disabled={busy}
            class="h-7 w-7 rounded text-ink-300 hover:text-accent-red hover:bg-tint-strong grid place-items-center disabled:opacity-50"
            aria-label={`Remove ${user.email}`}
            title="Remove user"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </div>
  );
}
