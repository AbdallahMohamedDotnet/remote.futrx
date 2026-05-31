import { useEffect, useMemo, useState } from "preact/hooks";
import { ProjectContainersPage } from "../components/projects/ProjectContainersPage";
import type { ProjectContainerInfo, ProjectMeta } from "../models/project";
import { projectService } from "../services/projectService";

export interface ProjectContainerRecord {
  loading: boolean;
  data?: ProjectContainerInfo;
  error?: string;
  refreshedAt?: number;
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

  const [record, setRecord] = useState<ProjectContainerRecord>({ loading: false });
  const [refreshing, setRefreshing] = useState(false);

  async function load(signal?: { cancelled: boolean }) {
    if (!selectedProject) {
      setRecord({ loading: false });
      return;
    }
    setRefreshing(true);
    setRecord((prev) => ({ ...prev, loading: true, error: undefined }));
    try {
      const data = await projectService.containerInfo(selectedProject.id);
      if (signal?.cancelled) return;
      setRecord({ loading: false, data, refreshedAt: Date.now() });
    } catch (error) {
      if (signal?.cancelled) return;
      setRecord({
        loading: false,
        error: (error as Error).message,
        refreshedAt: Date.now(),
      });
    } finally {
      if (!signal?.cancelled) setRefreshing(false);
    }
  }

  useEffect(() => {
    const signal = { cancelled: false };
    void load(signal);
    return () => {
      signal.cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedProject?.id]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      record={record}
      refreshing={refreshing}
      onRefresh={() => void load()}
      onBack={onBack}
      onHamburger={onHamburger}
    />
  );
}
