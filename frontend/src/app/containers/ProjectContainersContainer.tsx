import { useCallback } from "preact/hooks";
import { ProjectContainersPage } from "../../ui/projects/ProjectContainersPage";
import type { ProjectMeta } from "../../models/project";
import { useProjectContainersController } from "../../state/hooks/projects/useProjectContainersController";

export function ProjectContainersContainer({
  projects,
  selectedProjectId,
  onBack,
  onHamburger,
  onDeleteProject,
}: {
  projects: ProjectMeta[];
  selectedProjectId: string | null;
  onBack: () => void;
  onHamburger: () => void;
  onDeleteProject: (projectId: string) => Promise<void>;
}) {
  const controller = useProjectContainersController(projects, selectedProjectId);
  const { selectedProject, info, secrets, access } = controller;

  const deleteSelectedProject = useCallback(async () => {
    if (!selectedProject) return;
    await onDeleteProject(selectedProject.id);
    onBack();
  }, [selectedProject, onBack, onDeleteProject]);

  return (
    <ProjectContainersPage
      project={selectedProject}
      infoRecord={info.record}
      secretsRecord={secrets.record}
      accessRecord={access.record}
      refreshing={controller.refreshing}
      onRefresh={() => void controller.refresh()}
      onBack={onBack}
      onHamburger={onHamburger}
      onSaveSecret={secrets.save}
      onDeleteSecret={secrets.remove}
      onAddMember={access.add}
      onRemoveMember={access.remove}
      onRepairNetwork={info.repairNetwork}
      onStartProject={info.start}
      onStopProject={info.stop}
      onDeleteProject={deleteSelectedProject}
    />
  );
}
