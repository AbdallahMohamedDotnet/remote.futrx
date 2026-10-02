import type { RbacAssignment } from "../../../models/rbac";
import { PERMISSIONS_EMPTY_COPY } from "../../../state/hooks/permissions/permissionsView";
import { Empty } from "../../projects/project-containers/ProjectContainerPrimitives";
import { EffectBadge, PermissionsRow, PermissionsSection, ScopeBadge } from "./PermissionsPrimitives";

export function AssignmentsList({
  assignments,
  loading,
}: {
  assignments: RbacAssignment[];
  loading: boolean;
}) {
  return (
    <PermissionsSection
      title="Assignments"
      description="Permissions granted or denied directly to a user."
      loading={loading}
    >
      {assignments.length === 0 ? (
        <Empty text={PERMISSIONS_EMPTY_COPY.assignments} compact />
      ) : (
        assignments.map((assignment) => (
          <PermissionsRow key={assignment.id}>
            <span class="text-[12.5px] text-ink-50 truncate" title={assignment.userEmail}>
              {assignment.userEmail}
            </span>
            <span class="font-mono text-[12px] text-ink-200 truncate" title={assignment.permission}>
              {assignment.permission}
            </span>
            <EffectBadge effect={assignment.effect} />
            <ScopeBadge scope={assignment.scope} />
          </PermissionsRow>
        ))
      )}
    </PermissionsSection>
  );
}
