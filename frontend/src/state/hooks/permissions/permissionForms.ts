import type {
  RbacAssignmentInput,
  RbacBindingInput,
  RbacDefinition,
  RbacEffect,
  RbacRole,
  RbacScope,
  RbacScopeKind,
} from "../../../models/rbac";

export type FormResult<T> = { ok: true; input: T } | { ok: false; message: string };

export interface AssignmentDraft {
  userEmail: string;
  permission: string;
  effect: RbacEffect;
  scopeKind: RbacScopeKind;
  projectId: string;
}

export interface BindingDraft {
  userEmail: string;
  roleId: string;
  scopeKind: RbacScopeKind;
  projectId: string;
}

const EMAIL_PATTERN = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

class PermissionFormLogic {
  /** Scope kinds the permission is registered for; the definitions list is the only source. */
  scopeKindsForPermission(definitions: RbacDefinition[], key: string): RbacScopeKind[] {
    return definitions.find((definition) => definition.key === key)?.scopes ?? [];
  }

  /** Scope kinds every rule of the role supports, mirroring the server's bind-time check. */
  scopeKindsForRole(definitions: RbacDefinition[], role: RbacRole | undefined): RbacScopeKind[] {
    if (!role || role.rules.length === 0) return [];
    const [first, ...rest] = role.rules.map((rule) =>
      this.scopeKindsForPermission(definitions, rule.permission)
    );
    return first.filter((kind) => rest.every((kinds) => kinds.includes(kind)));
  }

  /** The kind to show: the chosen one when still offered, else the first offered. */
  effectiveScopeKind(offered: RbacScopeKind[], chosen: RbacScopeKind): RbacScopeKind {
    return offered.includes(chosen) ? chosen : offered[0] ?? chosen;
  }

  buildScope(kind: RbacScopeKind, projectId: string): RbacScope {
    return kind === "project" ? { kind, id: projectId } : { kind };
  }

  validateAssignment(
    draft: AssignmentDraft,
    definitions: RbacDefinition[]
  ): FormResult<RbacAssignmentInput> {
    const userEmail = draft.userEmail.trim().toLowerCase();
    if (!userEmail) return { ok: false, message: "Email is required." };
    if (!EMAIL_PATTERN.test(userEmail)) return { ok: false, message: "That doesn't look like an email." };
    if (!draft.permission) return { ok: false, message: "Choose a permission." };
    const offered = this.scopeKindsForPermission(definitions, draft.permission);
    const kind = this.effectiveScopeKind(offered, draft.scopeKind);
    if (!offered.includes(kind)) return { ok: false, message: "That permission is not registered." };
    if (kind === "project" && !draft.projectId) return { ok: false, message: "Choose a project." };
    return {
      ok: true,
      input: {
        userEmail,
        permission: draft.permission,
        effect: draft.effect,
        scope: this.buildScope(kind, draft.projectId),
      },
    };
  }

  validateBinding(
    draft: BindingDraft,
    roles: RbacRole[],
    definitions: RbacDefinition[]
  ): FormResult<RbacBindingInput> {
    const userEmail = draft.userEmail.trim().toLowerCase();
    if (!userEmail) return { ok: false, message: "Email is required." };
    if (!EMAIL_PATTERN.test(userEmail)) return { ok: false, message: "That doesn't look like an email." };
    const role = roles.find((candidate) => candidate.id === draft.roleId);
    if (!role) return { ok: false, message: "Choose a role." };
    const offered = this.scopeKindsForRole(definitions, role);
    const kind = this.effectiveScopeKind(offered, draft.scopeKind);
    if (!offered.includes(kind)) {
      return { ok: false, message: "This role's rules share no scope kind." };
    }
    if (kind === "project" && !draft.projectId) return { ok: false, message: "Choose a project." };
    return {
      ok: true,
      input: { userEmail, roleId: role.id, scope: this.buildScope(kind, draft.projectId) },
    };
  }
}

export const permissionForms = new PermissionFormLogic();
