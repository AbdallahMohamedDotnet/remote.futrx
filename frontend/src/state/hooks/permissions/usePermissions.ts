import { useCallback, useEffect, useState } from "preact/hooks";
import { permissionsApi } from "../../../api/permissionsApi";
import type {
  RbacAssignment,
  RbacBinding,
  RbacDefinition,
  RbacRole,
} from "../../../models/rbac";

export interface PermissionsController {
  loading: boolean;
  definitions: RbacDefinition[];
  roles: RbacRole[];
  assignments: RbacAssignment[];
  bindings: RbacBinding[];
  error: string | null;
  refresh: () => Promise<void>;
}

export function usePermissions(enabled: boolean): PermissionsController {
  const [loading, setLoading] = useState(false);
  const [definitions, setDefinitions] = useState<RbacDefinition[]>([]);
  const [roles, setRoles] = useState<RbacRole[]>([]);
  const [assignments, setAssignments] = useState<RbacAssignment[]>([]);
  const [bindings, setBindings] = useState<RbacBinding[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextDefinitions, state] = await Promise.all([
        permissionsApi.listDefinitions(),
        permissionsApi.fetchState(),
      ]);
      setDefinitions(nextDefinitions);
      setRoles(state.roles);
      setAssignments(state.assignments);
      setBindings(state.bindings);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (enabled) void refresh();
  }, [enabled, refresh]);

  return { loading, definitions, roles, assignments, bindings, error, refresh };
}
