"use client";

import { useEffect, useRef } from "react";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { wsUrl } from "@/lib/backend";

interface TerminalPaneProps {
  paneId: string;
  alive: boolean;
  active: boolean;
  /** Called when the pane's process exits (so the parent can mark the tab dead). */
  onExit: () => void;
}

export default function TerminalPane({
  paneId,
  alive,
  active,
  onExit,
}: TerminalPaneProps) {
  const elRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  // Keep the latest onExit without re-running the mount effect.
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;

  // Mount xterm + wire the WebSocket once. xterm touches `window`, so it is
  // dynamically imported inside the effect — never at module scope, which would
  // break the static prerender.
  useEffect(() => {
    let disposed = false;
    let term: Terminal;
    let fit: FitAddon;
    let ws: WebSocket | null = null;

    const sendResize = () => {
      fit.fit();
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }),
        );
      }
    };

    (async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      if (disposed || !elRef.current) return;

      term = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: "monospace",
      });
      fit = new FitAddon();
      term.loadAddon(fit);
      term.open(elRef.current);
      fit.fit();

      termRef.current = term;
      fitRef.current = fit;

      if (!alive) {
        term.writeln("\r\n\x1b[2m[process exited]\x1b[0m");
        return;
      }

      ws = new WebSocket(wsUrl(`/ws/pane/${paneId}`));
      ws.binaryType = "arraybuffer";
      wsRef.current = ws;

      ws.onerror = () => term.writeln("\r\n\x1b[31m[WebSocket error]\x1b[0m");
      ws.onclose = (e) => {
        window.removeEventListener("resize", sendResize);
        if (!e.wasClean) term.writeln("\r\n\x1b[31m[connection lost]\x1b[0m");
      };
      ws.onmessage = (e) => {
        if (typeof e.data === "string") {
          try {
            const msg = JSON.parse(e.data);
            if (msg.type === "exited") {
              onExitRef.current();
              term.writeln("\r\n\x1b[2m[process exited]\x1b[0m");
            }
          } catch {
            // ignore malformed control frames
          }
        } else {
          term.write(new Uint8Array(e.data as ArrayBuffer));
        }
      };

      term.onData((data) => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data));
        }
      });

      ws.onopen = sendResize;
      window.addEventListener("resize", sendResize);
    })();

    return () => {
      disposed = true;
      window.removeEventListener("resize", sendResize);
      wsRef.current?.close();
      wsRef.current = null;
      termRef.current?.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [paneId, alive]);

  // Refit when this pane becomes the active (visible) tab — fitting a hidden
  // (display:none) element measures zero.
  useEffect(() => {
    if (!active) return;
    const fit = fitRef.current;
    const term = termRef.current;
    const ws = wsRef.current;
    if (!fit || !term) return;
    fit.fit();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }),
      );
    }
  }, [active]);

  return (
    <div
      ref={elRef}
      className="h-full w-full"
      style={{ display: active ? "block" : "none" }}
    />
  );
}
