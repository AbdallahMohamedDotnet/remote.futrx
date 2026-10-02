import type { RbacRole } from "../../../models/rbac";
import { Empty } from "../../projects/project-containers/ProjectContainerPrimitives";
import { PermissionBadge, PermissionsRow, PermissionsSection } from "./PermissionsPrimitives";

export function RolesList({ roles, loading }: { roles: RbacRole[]; loading: boolean }) {
  return (
    <PermissionsSection
      title="Roles"
      description="Named sets of allow and deny rules that can be bound to users."
      loading={loading}
    >
      {roles.length === 0 ? (
        <Empty text="No roles defined yet." compact />
      ) : (
        roles.map((role) => (
          <PermissionsRow key={role.id}>
            <span class="text-[12.5px] text-ink-50 truncate" title={role.description || role.name}>
              {role.name}
            </span>
            <PermissionBadge tone="neutral">
              {`${role.rules.length} rule${role.rules.length === 1 ? "" : "s"}`}
            </PermissionBadge>
          </PermissionsRow>
        ))
      )}
    </PermissionsSection>
  );
}
