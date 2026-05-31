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

export type ProjectContainerRecords = Record<string, ProjectContainerRecord>;

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
  const [records, setRecords] = useState<ProjectContainerRecords>({});
  const [refreshing, setRefreshing] = useState(false);
  const projectIds = useMemo(() => projects.map((project) => project.id).join("|"), [projects]);

  async function load(signal?: { cancelled: boolean }) {
    if (projects.length === 0) {
      setRecords({});
      return;
    }

    setRefreshing(true);
    setRecords((current) => {
      const next: ProjectContainerRecords = {};
      for (const project of projects) {
        next[project.id] = { ...current[project.id], loading: true, error: undefined };
      }
      return next;
    });

    const entries = await Promise.all(
      projects.map(async (project): Promise<[string, ProjectContainerRecord]> => {
        try {
          const data = await projectService.containerInfo(project.id);
          return [project.id, { loading: false, data, refreshedAt: Date.now() }];
        } catch (error) {
          return [
            project.id,
            {
              loading: false,
              error: (error as Error).message,
              refreshedAt: Date.now(),
            },
          ];
        }
      })
    );

    if (signal?.cancelled) return;
    setRecords(Object.fromEntries(entries));
    setRefreshing(false);
  }

  useEffect(() => {
    const signal = { cancelled: false };
    void load(signal);
    return () => {
      signal.cancelled = true;
    };
    // Projects are loaded by workspace state; projectIds is the stable fetch boundary.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectIds]);

  const orderedProjects = useMemo(() => {
    if (!selectedProjectId) return projects;
    const selected = projects.find((project) => project.id === selectedProjectId);
    if (!selected) return projects;
    return [selected, ...projects.filter((project) => project.id !== selectedProjectId)];
  }, [projects, selectedProjectId]);

  return (
    <ProjectContainersPage
      projects={orderedProjects}
      selectedProjectId={selectedProjectId}
      records={records}
      refreshing={refreshing}
      onRefresh={() => void load()}
      onBack={onBack}
      onHamburger={onHamburger}
    />
  );
}
