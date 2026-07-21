import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { ProjectContainersPage } from "../../ui/projects/ProjectContainersPage";
import type { ProjectMeta } from "../../models/project";
import { projectApi } from "../../api/projectApi";
import type {
  AccessRecord,
  ProjectContainerRecord,
  SecretsRecord,
} from "../../state/projects/projectContainerRecords";

export function ProjectContainersContainer({
  projects,
  selectedProjectId,
  onBack,
  onHamburger,
}: {
  projects: ProjectMeta[];
  selectedProjectId: string | null;
  onBack: () => void;
  onHamburger: () => void;
}) {
  const selectedProject = useMemo(
    () => projects.find((p) => p.id === selectedProjectId) ?? null,
    [projects, selectedProjectId]
  );

  const [infoRecord, setInfoRecord] = useState<ProjectContainerRecord>({ loading: false });
  const [secretsRecord, setSecretsRecord] = useState<SecretsRecord>({ loading: false });
  const [accessRecord, setAccessRecord] = useState<AccessRecord>({ loading: false });
  const [refreshing, setRefreshing] = useState(false);

  const loadInfo = useCallback(
    async (signal?: { cancelled: boolean }) => {
      if (!selectedProject) {
        setInfoRecord({ loading: false });
        return;
      }
      setInfoRecord((prev) => ({ ...prev, loading: true, error: undefined }));
      try {
        const data = await projectApi.containerInfo(selectedProject.id);
        if (signal?.cancelled) return;
        setInfoRecord({ loading: false, data, refreshedAt: Date.now() });
      } catch (error) {
        if (signal?.cancelled) return;
        setInfoRecord({
          loading: false,
          error: (error as Error).message,
          refreshedAt: Date.now(),
        });
      }
    },
    [selectedProject]
  );

  const loadSecrets = useCallback(
    async (signal?: { cancelled: boolean }) => {
      if (!selectedProject) {
        setSecretsRecord({ loading: false });
        return;
      }
      setSecretsRecord((prev) => ({ ...prev, loading: true, error: undefined }));
      try {
        const data = await projectApi.listSecrets(selectedProject.id);
        if (signal?.cancelled) return;
        setSecretsRecord({ loading: false, data });
      } catch (error) {
        if (signal?.cancelled) return;
        setSecretsRecord({ loading: false, error: (error as Error).message });
      }
    },
    [selectedProject]
  );

  const loadAccess = useCallback(
    async (signal?: { cancelled: boolean }) => {
      if (!selectedProject) {
        setAccessRecord({ loading: false });
        return;
      }
      setAccessRecord((prev) => ({ ...prev, loading: true, error: undefined }));
      try {
        const data = await projectApi.listAccess(selectedProject.id);
        if (signal?.cancelled) return;
        setAccessRecord({ loading: false, data });
      } catch (error) {
        if (signal?.cancelled) return;
        setAccessRecord({ loading: false, error: (error as Error).message });
      }
    },
    [selectedProject]
  );

  const refresh = useCallback(async () => {
    if (!selectedProject) return;
    setRefreshing(true);
    try {
      await Promise.all([loadInfo(), loadSecrets(), loadAccess()]);
    } finally {
      setRefreshing(false);
    }
  }, [selectedProject, loadInfo, loadSecrets, loadAccess]);

  const onSaveSecret = useCallback(
    async (key: string, value: string) => {
      if (!selectedProject) return;
      const saved = await projectApi.setSecret(selectedProject.id, key, value);
      setSecretsRecord((prev) => {
        const list = prev.data ? [...prev.data] : [];
        const idx = list.findIndex((s) => s.key === saved.key);
        if (idx >= 0) list[idx] = saved;
        else list.push(saved);
        list.sort((a, b) => a.key.localeCompare(b.key));
        return { loading: false, data: list };
      });
    },
    [selectedProject]
  );

  const onDeleteSecret = useCallback(
    async (key: string) => {
      if (!selectedProject) return;
      await projectApi.deleteSecret(selectedProject.id, key);
      setSecretsRecord((prev) => ({
        loading: false,
        data: prev.data?.filter((s) => s.key !== key) ?? [],
      }));
    },
    [selectedProject]
  );

  const onAddMember = useCallback(
    async (email: string) => {
      if (!selectedProject) return;
      const { email: added } = await projectApi.addAccess(selectedProject.id, email);
      setAccessRecord((prev) => {
        const next = prev.data ? [...prev.data] : [];
        if (!next.includes(added)) next.push(added);
        next.sort();
        return { loading: false, data: next };
      });
    },
    [selectedProject]
  );

  const onRemoveMember = useCallback(
    async (email: string) => {
      if (!selectedProject) return;
      await projectApi.removeAccess(selectedProject.id, email);
      setAccessRecord((prev) => ({
        loading: false,
        data: prev.data?.filter((m) => m !== email) ?? [],
      }));
    },
    [selectedProject]
  );

  const onRepairNetwork = useCallback(async () => {
    if (!selectedProject) return;
    const data = await projectApi.repairNetwork(selectedProject.id);
    setInfoRecord({ loading: false, data, refreshedAt: Date.now() });
  }, [selectedProject]);

  const onStartProject = useCallback(async () => {
    if (!selectedProject) return;
    await projectApi.start(selectedProject.id);
    await loadInfo();
  }, [selectedProject, loadInfo]);

  const onStopProject = useCallback(async () => {
    if (!selectedProject) return;
    await projectApi.stop(selectedProject.id);
    await loadInfo();
  }, [selectedProject, loadInfo]);

  const onDeleteProject = useCallback(async () => {
    if (!selectedProject) return;
    await projectApi.delete(selectedProject.id);
    onBack();
  }, [selectedProject, onBack]);

  useEffect(() => {
    const signal = { cancelled: false };
    void loadInfo(signal);
    void loadSecrets(signal);
    void loadAccess(signal);
    return () => {
      signal.cancelled = true;
    };
  }, [loadInfo, loadSecrets, loadAccess]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      infoRecord={infoRecord}
      secretsRecord={secretsRecord}
      accessRecord={accessRecord}
      refreshing={refreshing}
      onRefresh={() => void refresh()}
      onBack={onBack}
      onHamburger={onHamburger}
      onSaveSecret={onSaveSecret}
      onDeleteSecret={onDeleteSecret}
      onAddMember={onAddMember}
      onRemoveMember={onRemoveMember}
      onRepairNetwork={onRepairNetwork}
      onStartProject={onStartProject}
      onStopProject={onStopProject}
      onDeleteProject={onDeleteProject}
    />
  );
}
