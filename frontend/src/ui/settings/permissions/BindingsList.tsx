import type { RbacBinding, RbacRole } from "../../../models/rbac";
import { PERMISSIONS_EMPTY_COPY } from "../../../state/hooks/permissions/permissionsView";
import { Empty } from "../../projects/project-containers/ProjectContainerPrimitives";
import { PermissionBadge, PermissionsRow, PermissionsSection, ScopeBadge } from "./PermissionsPrimitives";

export function BindingsList({
  bindings,
  roles,
  loading,
}: {
  bindings: RbacBinding[];
  roles: RbacRole[];
  loading: boolean;
}) {
  return (
    <PermissionsSection
      title="Role bindings"
      description="Roles bound to a user at a platform or project scope."
      loading={loading}
    >
      {bindings.length === 0 ? (
        <Empty text={PERMISSIONS_EMPTY_COPY.bindings} compact />
      ) : (
        bindings.map((binding) => (
          <PermissionsRow key={binding.id}>
            <span class="text-[12.5px] text-ink-50 truncate" title={binding.userEmail}>
              {binding.userEmail}
            </span>
            <PermissionBadge tone="highlight">
              {roles.find((role) => role.id === binding.roleId)?.name ?? binding.roleId}
            </PermissionBadge>
            <ScopeBadge scope={binding.scope} />
          </PermissionsRow>
        ))
      )}
    </PermissionsSection>
  );
}
