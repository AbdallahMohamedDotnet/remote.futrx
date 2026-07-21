import { useCallback, useEffect, useState } from "preact/hooks";
import { userApi } from "../../../api/userApi";
import type { User, UserRole } from "../../../models/user";

export interface UserDirectory {
  users: User[] | null;
  loading: boolean;
  error: string | null;
  add: (email: string, role: UserRole) => Promise<void>;
  remove: (email: string) => Promise<void>;
  setRole: (email: string, role: UserRole) => Promise<void>;
}

export function useUserDirectory(enabled: boolean): UserDirectory {
  const [users, setUsers] = useState<User[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setUsers(await userApi.list());
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (enabled) void load();
  }, [enabled, load]);

  async function add(email: string, role: UserRole): Promise<void> {
    const created = await userApi.add({ email, role });
    setUsers((current) => {
      const next = current ? [...current] : [];
      next.push(created);
      next.sort((left, right) => left.email.localeCompare(right.email));
      return next;
    });
  }

  async function remove(email: string): Promise<void> {
    await userApi.remove(email);
    setUsers((current) => current?.filter((user) => user.email !== email) ?? []);
  }

  async function setRole(email: string, role: UserRole): Promise<void> {
    const updated = await userApi.setRole(email, role);
    setUsers((current) =>
      current?.map((user) =>
        user.email === email ? { ...user, role: updated.role } : user
      ) ?? []
    );
  }

  return { users, loading, error, add, remove, setRole };
}
