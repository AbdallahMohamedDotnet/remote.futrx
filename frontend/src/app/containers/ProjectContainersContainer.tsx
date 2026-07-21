import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { ProjectContainersPage } from "../../ui/projects/ProjectContainersPage";
import type { ProjectMeta } from "../../models/project";
import { projectApi } from "../../api/projectApi";
import { useProjectContainerInfo } from "../../state/hooks/projects/useProjectContainerInfo";
import { useProjectSecrets } from "../../state/hooks/projects/useProjectSecrets";
import type { AccessRecord } from "../../state/projects/projectContainerRecords";

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

  const info = useProjectContainerInfo(selectedProject);
  const secrets = useProjectSecrets(selectedProject);
  const [accessRecord, setAccessRecord] = useState<AccessRecord>({ loading: false });
  const [refreshing, setRefreshing] = useState(false);

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
      await Promise.all([info.load(), secrets.load(), loadAccess()]);
    } finally {
      setRefreshing(false);
    }
  }, [selectedProject, info.load, secrets.load, loadAccess]);

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

  const onDeleteProject = useCallback(async () => {
    if (!selectedProject) return;
    await projectApi.delete(selectedProject.id);
    onBack();
  }, [selectedProject, onBack]);

  useEffect(() => {
    const signal = { cancelled: false };
    void info.load(signal);
    void secrets.load(signal);
    void loadAccess(signal);
    return () => {
      signal.cancelled = true;
    };
  }, [info.load, secrets.load, loadAccess]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      infoRecord={info.record}
      secretsRecord={secrets.record}
      accessRecord={accessRecord}
      refreshing={refreshing}
      onRefresh={() => void refresh()}
      onBack={onBack}
      onHamburger={onHamburger}
      onSaveSecret={secrets.save}
      onDeleteSecret={secrets.remove}
      onAddMember={onAddMember}
      onRemoveMember={onRemoveMember}
      onRepairNetwork={info.repairNetwork}
      onStartProject={info.start}
      onStopProject={info.stop}
      onDeleteProject={onDeleteProject}
    />
  );
}
