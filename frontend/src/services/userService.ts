import { json } from "../api/http";
import type { User, UserRole } from "../models/user";

export const userService = {
  list: () => json<User[]>("GET", "/api/admin/users"),
  add: (input: { email: string; role: UserRole }) =>
    json<User>("POST", "/api/admin/users", input),
  remove: (email: string) =>
    json<{ ok: boolean }>(
      "DELETE",
      `/api/admin/users/${encodeURIComponent(email)}`
    ),
  setRole: (email: string, role: UserRole) =>
    json<User>(
      "PUT",
      `/api/admin/users/${encodeURIComponent(email)}/role`,
      { role }
    ),
};
