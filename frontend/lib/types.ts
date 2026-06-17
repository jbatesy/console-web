// Mirrors the JSON shapes emitted by the Go backend (internal/db/store.go).

export interface Command {
  label: string;
  template: string;
}

export interface Variable {
  name: string;
  regex: string;
  description: string;
}

export interface Job {
  id: string;
  name: string;
  commands: Command[];
  variables: Variable[];
}

export interface Pane {
  id: string;
  session_id: string;
  cmd_index: number;
  pid: number;
  alive: boolean;
  output_path: string;
}

// Shape of GET /api/sessions/{id} (internal/api/handlers.go getSession).
export interface SessionResponse {
  id: string;
  job_id: string;
  vars: Record<string, string>;
  panes: Pane[];
}
