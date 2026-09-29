"use client";

import React, { createContext, useContext } from "react";

export const projectScopeStorageKey = "fluxa:selected-project-id";

type ProjectScopeValue = {
  selectedProjectId: string;
  setSelectedProjectId: (projectId: string) => void;
};

const ProjectScopeContext = createContext<ProjectScopeValue>({
  selectedProjectId: "",
  setSelectedProjectId: () => undefined
});

export function ProjectScopeProvider({ value, children }: { value: ProjectScopeValue; children: React.ReactNode }) {
  return <ProjectScopeContext.Provider value={value}>{children}</ProjectScopeContext.Provider>;
}

export function useProjectScope() {
  return useContext(ProjectScopeContext);
}
