import { requestJson } from "./apiRequest";
import type {
  RbacAssignment,
  RbacAssignmentInput,
  RbacBinding,
  RbacBindingInput,
  RbacDefinition,
  RbacRole,
  RbacRoleInput,
  RbacState,
} from "../models/rbac";
import { API_ROUTES } from "../config/routes";

export const permissionsApi = {
  listDefinitions: () =>
    requestJson<RbacDefinition[]>("GET", API_ROUTES.permissions.definitions),
  fetchState: () => requestJson<RbacState>("GET", API_ROUTES.permissions.state),
  createRole: (input: RbacRoleInput) =>
    requestJson<RbacRole>("POST", API_ROUTES.permissions.roles, input),
  updateRole: (id: string, input: RbacRoleInput) =>
    requestJson<RbacRole>("PATCH", API_ROUTES.permissions.role(id), input),
  deleteRole: (id: string) =>
    requestJson<{ ok: boolean }>("DELETE", API_ROUTES.permissions.role(id)),
  setAssignment: (input: RbacAssignmentInput) =>
    requestJson<RbacAssignment>("POST", API_ROUTES.permissions.assignments, input),
  removeAssignment: (input: Omit<RbacAssignmentInput, "effect">) =>
    requestJson<void>("DELETE", API_ROUTES.permissions.assignments, input),
  bindRole: (input: RbacBindingInput) =>
    requestJson<RbacBinding>("POST", API_ROUTES.permissions.bindings, input),
  unbindRole: (input: RbacBindingInput) =>
    requestJson<void>("DELETE", API_ROUTES.permissions.bindings, input),
};
