import { requestJson } from "./apiRequest";
import type { User, UserRole } from "../models/user";
import { API_ROUTES } from "../config/routes";

export const userApi = {
  list: () => requestJson<User[]>("GET", API_ROUTES.users.collection),
  add: (input: { email: string; role: UserRole }) =>
    requestJson<User>("POST", API_ROUTES.users.collection, input),
  remove: (email: string) =>
    requestJson<{ ok: boolean }>("DELETE", API_ROUTES.users.item(email)),
  setRole: (email: string, role: UserRole) =>
    requestJson<User>("PUT", API_ROUTES.users.role(email), { role }),
};
