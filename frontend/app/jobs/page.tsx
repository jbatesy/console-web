"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  createJob,
  deleteJob,
  listJobs,
  updateJob,
} from "@/lib/api";
import type { Command, Job, Variable } from "@/lib/types";

const emptyJob: Job = { id: "", name: "", commands: [], variables: [] };

export default function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([]);
  // `form` null = nothing open (empty state). `selectedId` null while `form` is
  // set means a new (unsaved) job, so the ID field stays editable.
  const [form, setForm] = useState<Job | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [origin, setOrigin] = useState("");

  const isNew = form !== null && selectedId === null;

  const refreshJobs = useCallback(async () => {
    try {
      setJobs(await listJobs());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    setOrigin(window.location.origin);
    refreshJobs();
  }, [refreshJobs]);

  function selectJob(job: Job) {
    setError(null);
    setSelectedId(job.id);
    setForm({
      id: job.id,
      name: job.name,
      commands: (job.commands ?? []).map((c) => ({ ...c })),
      variables: (job.variables ?? []).map((v) => ({ ...v })),
    });
  }

  function newJob() {
    setError(null);
    setSelectedId(null);
    setForm({ ...emptyJob, commands: [], variables: [] });
  }

  // --- form field updaters ---------------------------------------------------
  function patch(p: Partial<Job>) {
    setForm((f) => (f ? { ...f, ...p } : f));
  }
  function patchCmd(i: number, p: Partial<Command>) {
    setForm((f) =>
      f
        ? { ...f, commands: f.commands.map((c, j) => (j === i ? { ...c, ...p } : c)) }
        : f,
    );
  }
  function patchVar(i: number, p: Partial<Variable>) {
    setForm((f) =>
      f
        ? { ...f, variables: f.variables.map((v, j) => (j === i ? { ...v, ...p } : v)) }
        : f,
    );
  }

  // --- actions ---------------------------------------------------------------
  async function save() {
    if (!form) return;
    const job: Job = {
      id: form.id.trim(),
      name: form.name.trim(),
      commands: form.commands,
      variables: form.variables,
    };
    try {
      const saved = isNew ? await createJob(job) : await updateJob(job);
      setForm(saved);
      setSelectedId(saved.id);
      setError(null);
      await refreshJobs();
    } catch (e) {
      setError(`Save failed: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  async function remove() {
    if (!form || isNew) return;
    if (!window.confirm(`Delete job "${form.id}"?`)) return;
    try {
      await deleteJob(form.id);
      setForm(null);
      setSelectedId(null);
      setError(null);
      await refreshJobs();
    } catch (e) {
      const msg =
        e instanceof ApiError && e.status === 409
          ? e.message
          : `Delete failed: ${e instanceof Error ? e.message : String(e)}`;
      setError(msg);
    }
  }

  const launchUrl = form
    ? `${origin}/?job=${encodeURIComponent(form.id)}${form.variables
        .filter((v) => v.name)
        .map((v) => `&${encodeURIComponent(v.name)}=__${v.name}__`)
        .join("")}`
    : "";

  function copyUrl() {
    if (launchUrl && navigator.clipboard) navigator.clipboard.writeText(launchUrl);
  }

  const inputCls =
    "w-full rounded border border-white/15 bg-black/30 px-2 py-1 text-sm outline-none focus:border-[var(--accent)]";

  return (
    <div className="flex h-[calc(100vh-41px)]">
      <aside className="flex w-64 shrink-0 flex-col border-r border-white/10">
        <div className="flex-1 overflow-y-auto">
          {jobs.map((j) => (
            <button
              key={j.id}
              onClick={() => selectJob(j)}
              className={`block w-full truncate px-4 py-2 text-left text-sm ${
                selectedId === j.id ? "bg-white/10" : "hover:bg-white/5"
              }`}
            >
              {j.name || j.id}
            </button>
          ))}
        </div>
        <button
          onClick={newJob}
          className="m-2 rounded bg-[var(--accent)]/20 px-3 py-2 text-sm text-[var(--accent)] hover:bg-[var(--accent)]/30"
        >
          + New Job
        </button>
      </aside>

      <main className="flex-1 overflow-y-auto p-6">
        {error && (
          <div className="mb-4 rounded border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm text-red-300">
            {error}
          </div>
        )}

        {!form ? (
          <p className="text-sm opacity-60">Select a job or create a new one.</p>
        ) : (
          <div className="max-w-3xl space-y-5">
            <h2 className="text-lg font-semibold">
              {isNew ? "New Job" : "Edit Job"}
            </h2>

            <div className="space-y-1">
              <label className="block text-xs uppercase opacity-60">
                ID (slug)
              </label>
              <input
                className={inputCls}
                value={form.id}
                readOnly={!isNew}
                style={!isNew ? { opacity: 0.5 } : undefined}
                placeholder="my-job"
                onChange={(e) => patch({ id: e.target.value })}
              />
            </div>

            <div className="space-y-1">
              <label className="block text-xs uppercase opacity-60">Name</label>
              <input
                className={inputCls}
                value={form.name}
                placeholder="My Job"
                onChange={(e) => patch({ name: e.target.value })}
              />
            </div>

            <section className="space-y-2">
              <div className="flex items-center gap-3">
                <span className="text-xs uppercase opacity-60">Commands</span>
                <button
                  className="text-xs text-[var(--accent)]"
                  onClick={() =>
                    patch({ commands: [...form.commands, { label: "", template: "" }] })
                  }
                >
                  + Add
                </button>
              </div>
              {form.commands.map((c, i) => (
                <div key={i} className="flex gap-2">
                  <input
                    className={`${inputCls} w-40`}
                    placeholder="Label"
                    value={c.label}
                    onChange={(e) => patchCmd(i, { label: e.target.value })}
                  />
                  <input
                    className={`${inputCls} font-mono`}
                    placeholder="Template: echo {{var}}"
                    value={c.template}
                    onChange={(e) => patchCmd(i, { template: e.target.value })}
                  />
                  <button
                    className="px-2 text-red-400 hover:text-red-300"
                    onClick={() =>
                      patch({ commands: form.commands.filter((_, j) => j !== i) })
                    }
                  >
                    ×
                  </button>
                </div>
              ))}
            </section>

            <section className="space-y-2">
              <div className="flex items-center gap-3">
                <span className="text-xs uppercase opacity-60">Variables</span>
                <button
                  className="text-xs text-[var(--accent)]"
                  onClick={() =>
                    patch({
                      variables: [
                        ...form.variables,
                        { name: "", regex: "", description: "" },
                      ],
                    })
                  }
                >
                  + Add
                </button>
              </div>
              {form.variables.map((v, i) => (
                <div key={i} className="flex gap-2">
                  <input
                    className={`${inputCls} w-32`}
                    placeholder="name"
                    value={v.name}
                    onChange={(e) => patchVar(i, { name: e.target.value })}
                  />
                  <input
                    className={`${inputCls} font-mono`}
                    placeholder="^regex$"
                    value={v.regex}
                    onChange={(e) => patchVar(i, { regex: e.target.value })}
                  />
                  <input
                    className={inputCls}
                    placeholder="description"
                    value={v.description}
                    onChange={(e) => patchVar(i, { description: e.target.value })}
                  />
                  <button
                    className="px-2 text-red-400 hover:text-red-300"
                    onClick={() =>
                      patch({ variables: form.variables.filter((_, j) => j !== i) })
                    }
                  >
                    ×
                  </button>
                </div>
              ))}
            </section>

            <div className="break-all rounded border border-white/10 bg-black/30 px-3 py-2 font-mono text-xs opacity-80">
              {launchUrl}
            </div>

            <div className="flex gap-2">
              <button
                onClick={save}
                className="rounded bg-[var(--accent)] px-4 py-1.5 text-sm font-medium text-black"
              >
                Save
              </button>
              {!isNew && (
                <button
                  onClick={remove}
                  className="rounded bg-red-500/80 px-4 py-1.5 text-sm font-medium text-white hover:bg-red-500"
                >
                  Delete
                </button>
              )}
              <button
                onClick={copyUrl}
                className="rounded border border-white/15 px-4 py-1.5 text-sm hover:bg-white/5"
              >
                Copy URL
              </button>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
