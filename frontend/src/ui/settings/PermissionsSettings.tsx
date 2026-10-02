import type { PermissionsController } from "../../state/hooks/permissions/usePermissions";
import { AlertCircle, Loader } from "../primitives/icons";
import { AssignmentsList } from "./permissions/AssignmentsList";
import { BindingsList } from "./permissions/BindingsList";
import { DefinitionsList } from "./permissions/DefinitionsList";
import { RolesList } from "./permissions/RolesList";

export function PermissionsSettings({ permissions }: { permissions: PermissionsController }) {
  const { loading, error, definitions, roles, assignments, bindings } = permissions;

  if (loading && !definitions.length) {
    return (
      <div class="flex items-center gap-2 text-[13px] text-ink-300">
        <Loader class="w-4 h-4 animate-spin" /> Loading permissions…
      </div>
    );
  }

  return (
    <div class="space-y-4">
      {error && (
        <div class="flex items-start gap-2.5 rounded-lg border border-accent-red/30 bg-accent-red/[0.08] px-3 py-2.5 text-[13px]">
          <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-red" />
          <div class="text-accent-red break-words">{error}</div>
        </div>
      )}
      <RolesList roles={roles} loading={loading} />
      <AssignmentsList assignments={assignments} loading={loading} />
      <BindingsList bindings={bindings} roles={roles} loading={loading} />
      <DefinitionsList definitions={definitions} />
    </div>
  );
}
