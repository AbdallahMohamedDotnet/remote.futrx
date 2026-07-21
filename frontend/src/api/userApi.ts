import { requestJson } from "./apiRequest";
import type { User, UserRole } from "../models/user";

export const userApi = {
  list: () => requestJson<User[]>("GET", "/api/admin/users"),
  add: (input: { email: string; role: UserRole }) =>
    requestJson<User>("POST", "/api/admin/users", input),
  remove: (email: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      `/api/admin/users/${encodeURIComponent(email)}`
    ),
  setRole: (email: string, role: UserRole) =>
    requestJson<User>(
      "PUT",
      `/api/admin/users/${encodeURIComponent(email)}/role`,
      { role }
    ),
};
