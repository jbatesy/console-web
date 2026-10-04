// Mirrors the JSON shapes emitted by the Go backend (internal/db/store.go).

export interface CommandBranch {
  when?: Record<string, string>;
  default?: boolean;
  template: string;
}

export interface Command {
  label: string;
  template?: string;
  branches?: CommandBranch[];
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
  /** Unix seconds the process ended (0 while running). */
  ended_at: number;
  /** Unix seconds the retained output is discarded (0 while running). */
  expires_at: number;
  expired: boolean;
}

// Shape of GET /api/sessions/{id} (internal/api/handlers.go getSession).
export interface SessionResponse {
  id: string;
  job_id: string;
  vars: Record<string, string>;
  panes: Pane[];
}
