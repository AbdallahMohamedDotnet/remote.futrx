import { ProjectContainersContainer } from "../../state/containers/ProjectContainersContainer";
import type { ProjectMeta } from "../../models/project";

export function ProjectContainersRoute({
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
  return (
    <ProjectContainersContainer
      projects={projects}
      selectedProjectId={selectedProjectId}
      onBack={onBack}
      onHamburger={onHamburger}
    />
  );
}
