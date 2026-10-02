import { useCallback, useEffect, useState } from "preact/hooks";
import { permissionsApi } from "../../../api/permissions";
import type {
  RbacAssignment,
  RbacBinding,
  RbacDefinition,
  RbacRole,
} from "../../../models/rbac";

export interface PermissionsController {
  loading: boolean;
  loaded: boolean;
  definitions: RbacDefinition[];
  roles: RbacRole[];
  assignments: RbacAssignment[];
  bindings: RbacBinding[];
  error: string | null;
  refresh: () => Promise<void>;
}

export function usePermissions(enabled: boolean): PermissionsController {
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [definitions, setDefinitions] = useState<RbacDefinition[]>([]);
  const [roles, setRoles] = useState<RbacRole[]>([]);
  const [assignments, setAssignments] = useState<RbacAssignment[]>([]);
  const [bindings, setBindings] = useState<RbacBinding[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextDefinitions, nextRoles, nextAssignments, nextBindings] = await Promise.all([
        permissionsApi.listDefinitions(),
        permissionsApi.listRoles(),
        permissionsApi.listAssignments(),
        permissionsApi.listBindings(),
      ]);
      setDefinitions(nextDefinitions);
      setRoles(nextRoles);
      setAssignments(nextAssignments);
      setBindings(nextBindings);
      setLoaded(true);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (enabled) void refresh();
  }, [enabled, refresh]);

  return { loading, loaded, definitions, roles, assignments, bindings, error, refresh };
}
