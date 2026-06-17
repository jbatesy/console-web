"use client";

import { useCallback, useEffect, useState } from "react";
import { apiUrl } from "@/lib/backend";
import { getSessionId } from "@/lib/session";
import type { Job, Pane, SessionResponse } from "@/lib/types";
import TerminalPane from "@/components/TerminalPane";

interface PaneView extends Pane {
  label: string;
}

export default function Home() {
  const [panes, setPanes] = useState<PaneView[]>([]);
  const [active, setActive] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      const sessionId = getSessionId();
      if (!sessionId) {
        setLoaded(true);
        return;
      }

      try {
        const resp = await fetch(apiUrl(`/api/sessions/${sessionId}`));
        if (!resp.ok) {
          if (!cancelled) setError("Session not found.");
          return;
        }
        const data: SessionResponse = await resp.json();
        const basePanes = data.panes ?? [];

        // Resolve tab labels from the job's command list (by index).
        let labels: string[] = [];
        if (data.job_id) {
          const jobResp = await fetch(apiUrl(`/api/jobs/${data.job_id}`));
          if (jobResp.ok) {
            const job: Job = await jobResp.json();
            labels = (job.commands ?? []).map((c) => c.label);
          }
        }

        if (cancelled) return;
        setPanes(
          basePanes.map((p, idx) => ({
            ...p,
            label: labels[idx] || `Pane ${idx + 1}`,
          })),
        );
      } catch (err) {
        if (!cancelled) {
          setError(
            `Failed to load session: ${
              err instanceof Error ? err.message : String(err)
            }`,
          );
        }
      } finally {
        if (!cancelled) setLoaded(true);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  const markDead = useCallback((paneId: string) => {
    setPanes((prev) =>
      prev.map((p) => (p.id === paneId ? { ...p, alive: false } : p)),
    );
  }, []);

  if (error) {
    return (
      <main className="p-4">
        <div className="rounded border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-300">
          {error}
        </div>
      </main>
    );
  }

  if (loaded && panes.length === 0) {
    return (
      <main className="p-4">
        <p className="text-sm opacity-70">
          No active session. Launch a job from the{" "}
          <a href="/jobs/" className="text-[var(--accent)] underline">
            Jobs
          </a>{" "}
          page.
        </p>
      </main>
    );
  }

  return (
    <div className="flex h-[calc(100vh-41px)] flex-col">
      <div className="flex shrink-0 gap-1 border-b border-white/10 px-2 py-1">
        {panes.map((pane, idx) => (
          <button
            key={pane.id}
            onClick={() => setActive(idx)}
            className={`flex items-center gap-2 rounded px-3 py-1 text-sm ${
              idx === active ? "bg-white/10" : "hover:bg-white/5"
            }`}
          >
            <span
              className={`inline-block h-2 w-2 rounded-full ${
                pane.alive ? "bg-green-400" : "bg-zinc-500"
              }`}
            />
            {pane.label}
          </button>
        ))}
      </div>
      <div className="min-h-0 flex-1 p-2">
        {panes.map((pane, idx) => (
          <TerminalPane
            key={pane.id}
            paneId={pane.id}
            alive={pane.alive}
            active={idx === active}
            onExit={() => markDead(pane.id)}
          />
        ))}
      </div>
    </div>
  );
}
