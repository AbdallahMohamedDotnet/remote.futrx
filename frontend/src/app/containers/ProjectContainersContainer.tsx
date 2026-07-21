import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { ProjectContainersPage } from "../../ui/projects/ProjectContainersPage";
import type { ProjectMeta } from "../../models/project";
import { projectApi } from "../../api/projectApi";
import { useProjectAccess } from "../../state/hooks/projects/useProjectAccess";
import { useProjectContainerInfo } from "../../state/hooks/projects/useProjectContainerInfo";
import { useProjectSecrets } from "../../state/hooks/projects/useProjectSecrets";

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
  const access = useProjectAccess(selectedProject);
  const [refreshing, setRefreshing] = useState(false);

  const refresh = useCallback(async () => {
    if (!selectedProject) return;
    setRefreshing(true);
    try {
      await Promise.all([info.load(), secrets.load(), access.load()]);
    } finally {
      setRefreshing(false);
    }
  }, [selectedProject, info.load, secrets.load, access.load]);

  const onDeleteProject = useCallback(async () => {
    if (!selectedProject) return;
    await projectApi.delete(selectedProject.id);
    onBack();
  }, [selectedProject, onBack]);

  useEffect(() => {
    const signal = { cancelled: false };
    void info.load(signal);
    void secrets.load(signal);
    void access.load(signal);
    return () => {
      signal.cancelled = true;
    };
  }, [info.load, secrets.load, access.load]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      infoRecord={info.record}
      secretsRecord={secrets.record}
      accessRecord={access.record}
      refreshing={refreshing}
      onRefresh={() => void refresh()}
      onBack={onBack}
      onHamburger={onHamburger}
      onSaveSecret={secrets.save}
      onDeleteSecret={secrets.remove}
      onAddMember={access.add}
      onRemoveMember={access.remove}
      onRepairNetwork={info.repairNetwork}
      onStartProject={info.start}
      onStopProject={info.stop}
      onDeleteProject={onDeleteProject}
    />
  );
}
