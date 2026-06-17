import { apiUrl } from "./backend";
import type { Job } from "./types";

/** Error carrying the HTTP status so callers can special-case (e.g. 409). */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

async function fail(resp: Response): Promise<never> {
  const text = (await resp.text().catch(() => "")).trim();
  throw new ApiError(resp.status, text || `HTTP ${resp.status}`);
}

export async function listJobs(): Promise<Job[]> {
  const resp = await fetch(apiUrl("/api/jobs"));
  if (!resp.ok) return fail(resp);
  return resp.json();
}

export async function createJob(job: Job): Promise<Job> {
  const resp = await fetch(apiUrl("/api/jobs"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(job),
  });
  if (!resp.ok) return fail(resp);
  return resp.json();
}

export async function updateJob(job: Job): Promise<Job> {
  const resp = await fetch(apiUrl(`/api/jobs/${job.id}`), {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(job),
  });
  if (!resp.ok) return fail(resp);
  return resp.json();
}

export async function deleteJob(id: string): Promise<void> {
  const resp = await fetch(apiUrl(`/api/jobs/${id}`), { method: "DELETE" });
  if (!resp.ok) return fail(resp);
}
