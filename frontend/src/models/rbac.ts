export type RbacEffect = "allow" | "deny";
export type RbacScopeKind = "platform" | "project";

export interface RbacScope {
  kind: RbacScopeKind;
  id?: string;
}

export interface RbacRoleRule {
  permission: string;
  effect: RbacEffect;
}

export interface RbacDefinition {
  key: string;
  description: string;
  scopes: RbacScopeKind[];
  baseline: string;
  delegable: boolean;
}

export interface RbacRole {
  id: string;
  name: string;
  description: string;
  rules: RbacRoleRule[];
  createdBy: string;
  createdAt: number;
  updatedAt: number;
}

export interface RbacAssignment {
  id: string;
  userEmail: string;
  permission: string;
  effect: RbacEffect;
  scope: RbacScope;
  createdBy: string;
  createdAt: number;
}

export interface RbacBinding {
  id: string;
  roleId: string;
  userEmail: string;
  scope: RbacScope;
  createdBy: string;
  createdAt: number;
}

export interface RbacState {
  roles: RbacRole[];
  assignments: RbacAssignment[];
  bindings: RbacBinding[];
}

export interface RbacRoleInput {
  name: string;
  description: string;
  rules: RbacRoleRule[];
}

export interface RbacAssignmentInput {
  userEmail: string;
  permission: string;
  effect: RbacEffect;
  scope: RbacScope;
}

export interface RbacBindingInput {
  roleId: string;
  userEmail: string;
  scope: RbacScope;
}
