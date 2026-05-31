import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { ProjectContainersPage } from "../components/projects/ProjectContainersPage";
import type { ProjectContainerInfo, ProjectMeta, ProjectSecret } from "../models/project";
import { projectService } from "../services/projectService";

export interface ProjectContainerRecord {
  loading: boolean;
  data?: ProjectContainerInfo;
  error?: string;
  refreshedAt?: number;
}

export interface SecretsRecord {
  loading: boolean;
  data?: ProjectSecret[];
  error?: string;
}

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
  const [refreshing, setRefreshing] = useState(false);

  const loadInfo = useCallback(
    async (signal?: { cancelled: boolean }) => {
      if (!selectedProject) {
        setInfoRecord({ loading: false });
        return;
      }
      setInfoRecord((prev) => ({ ...prev, loading: true, error: undefined }));
      try {
        const data = await projectService.containerInfo(selectedProject.id);
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
        const data = await projectService.listSecrets(selectedProject.id);
        if (signal?.cancelled) return;
        setSecretsRecord({ loading: false, data });
      } catch (error) {
        if (signal?.cancelled) return;
        setSecretsRecord({ loading: false, error: (error as Error).message });
      }
    },
    [selectedProject]
  );

  const refresh = useCallback(async () => {
    if (!selectedProject) return;
    setRefreshing(true);
    try {
      await Promise.all([loadInfo(), loadSecrets()]);
    } finally {
      setRefreshing(false);
    }
  }, [selectedProject, loadInfo, loadSecrets]);

  const onSaveSecret = useCallback(
    async (key: string, value: string) => {
      if (!selectedProject) return;
      const saved = await projectService.setSecret(selectedProject.id, key, value);
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
      await projectService.deleteSecret(selectedProject.id, key);
      setSecretsRecord((prev) => ({
        loading: false,
        data: prev.data?.filter((s) => s.key !== key) ?? [],
      }));
    },
    [selectedProject]
  );

  useEffect(() => {
    const signal = { cancelled: false };
    void loadInfo(signal);
    void loadSecrets(signal);
    return () => {
      signal.cancelled = true;
    };
  }, [loadInfo, loadSecrets]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      infoRecord={infoRecord}
      secretsRecord={secretsRecord}
      refreshing={refreshing}
      onRefresh={() => void refresh()}
      onBack={onBack}
      onHamburger={onHamburger}
      onSaveSecret={onSaveSecret}
      onDeleteSecret={onDeleteSecret}
    />
  );
}
