import { useState } from "preact/hooks";
import type { UserRole } from "../../models/user";

export function AddUserForm({
  onAdd,
}: {
  onAdd: (email: string, role: UserRole) => Promise<void>;
}) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<UserRole>("member");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async (e: Event) => {
    e.preventDefault();
    const em = email.trim().toLowerCase();
    if (!em) {
      setErr("Email is required.");
      return;
    }
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(em)) {
      setErr("That doesn't look like an email.");
      return;
    }
    setErr(null);
    setSubmitting(true);
    try {
      await onAdd(em, role);
      setEmail("");
      setRole("member");
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      class="rounded-md border border-line bg-tint p-2.5 space-y-2"
    >
      <div class="grid gap-2 sm:grid-cols-[2fr_auto_auto] items-center">
        <input
          type="email"
          value={email}
          onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
          placeholder="someone@example.com"
          spellcheck={false}
          autoComplete="off"
          class="h-9 px-2.5 rounded border border-line bg-inset text-[13px] text-ink-50 placeholder-ink-400 focus:outline-none focus:border-accent-blue/50"
        />
        <select
          value={role}
          onChange={(e) => setRole((e.target as HTMLSelectElement).value as UserRole)}
          class="h-9 px-2 rounded border border-line bg-inset text-[13px] text-ink-50 focus:outline-none focus:border-accent-blue/50"
        >
          <option value="member">member</option>
          <option value="admin">admin</option>
        </select>
        <button
          type="submit"
          disabled={submitting}
          class="btn btn-primary btn-sm disabled:opacity-50"
        >
          {submitting ? "Adding…" : "Add"}
        </button>
      </div>
      {err && <div class="text-[11.5px] text-accent-red">{err}</div>}
    </form>
  );
}
