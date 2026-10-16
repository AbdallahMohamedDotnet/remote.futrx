import type { DirectoryUser, UserRole } from "../../models/user";
import { UserRow } from "./UserRow";

export function UserList({
  users,
  loading,
  currentEmail,
  onRemove,
  onSetRole,
}: {
  users: DirectoryUser[];
  loading: boolean;
  currentEmail: string;
  onRemove: (email: string) => Promise<void>;
  onSetRole: (email: string, role: UserRole) => Promise<void>;
}) {
  if (loading) {
    return (
      <div class="rounded-md border border-line bg-tint px-3 py-4 text-center text-[12.5px] text-ink-300">
        Loading users…
      </div>
    );
  }
  if (users.length === 0) {
    return (
      <div class="rounded-md border border-line bg-tint px-3 py-2.5 text-[13px] text-ink-300">
        No users yet.
      </div>
    );
  }
  return (
    <div class="space-y-2">
      {users.map((u) => (
        <UserRow
          key={u.email}
          user={u}
          isSelf={u.email === currentEmail.toLowerCase()}
          onRemove={() => onRemove(u.email)}
          onSetRole={(r) => onSetRole(u.email, r)}
        />
      ))}
    </div>
  );
}
