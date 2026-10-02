import { useCallback, useEffect, useState } from "preact/hooks";
import { permissionsApi } from "../../../api/permissions";
import type {
  RbacAssignment,
  RbacAssignmentInput,
  RbacBinding,
  RbacBindingInput,
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
  addAssignment: (input: RbacAssignmentInput) => Promise<void>;
  removeAssignment: (id: string) => Promise<void>;
  addBinding: (input: RbacBindingInput) => Promise<void>;
  removeBinding: (id: string) => Promise<void>;
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

  const addAssignment = useCallback(
    async (input: RbacAssignmentInput) => {
      await permissionsApi.addAssignment(input);
      await refresh();
    },
    [refresh]
  );

  const removeAssignment = useCallback(
    async (id: string) => {
      await permissionsApi.removeAssignment(id);
      await refresh();
    },
    [refresh]
  );

  const addBinding = useCallback(
    async (input: RbacBindingInput) => {
      await permissionsApi.addBinding(input);
      await refresh();
    },
    [refresh]
  );

  const removeBinding = useCallback(
    async (id: string) => {
      await permissionsApi.removeBinding(id);
      await refresh();
    },
    [refresh]
  );

  return {
    loading,
    loaded,
    definitions,
    roles,
    assignments,
    bindings,
    error,
    refresh,
    addAssignment,
    removeAssignment,
    addBinding,
    removeBinding,
  };
}
