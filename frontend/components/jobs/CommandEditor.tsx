"use client";

import {
  isConditional,
  resolveCommand,
  testVariableRegex,
} from "@/lib/branches";
import type { Command, CommandBranch, Variable } from "@/lib/types";
import { BranchEditor } from "./BranchEditor";

function emptyBranch(vars: Variable[]): CommandBranch {
  const first = vars.find((v) => v.name);
  return {
    when: first ? { [first.name]: "" } : {},
    template: "",
  };
}

type CommandEditorProps = {
  command: Command;
  namedVars: Variable[];
  testVars: Record<string, string>;
  onTestVarChange: (name: string, value: string) => void;
  inputCls: string;
  onChange: (command: Command) => void;
  onRemove: () => void;
};

export function CommandEditor({
  command,
  namedVars,
  testVars,
  onTestVarChange,
  inputCls,
  onChange,
  onRemove,
}: CommandEditorProps) {
  const conditional = isConditional(command);
  const hasElse = (command.branches ?? []).some((b) => b.default);
  const resolution = conditional ? resolveCommand(command, testVars) : null;

  function setMode(mode: "simple" | "conditional") {
    if (mode === "conditional" && !isConditional(command)) {
      onChange({
        ...command,
        branches: [{ when: {}, template: command.template ?? "" }],
        template: undefined,
      });
    } else if (mode === "simple" && isConditional(command)) {
      const fallback =
        command.branches?.find((b) => b.default)?.template ??
        command.branches?.[0]?.template ??
        "";
      onChange({ label: command.label, template: fallback });
    }
  }

  function patchBranch(branchIdx: number, patch: Partial<CommandBranch>) {
    const branches = [...(command.branches ?? [])];
    branches[branchIdx] = { ...branches[branchIdx], ...patch };
    onChange({ ...command, branches });
  }

  function addBranch() {
    onChange({
      ...command,
      branches: [...(command.branches ?? []), emptyBranch(namedVars)],
    });
  }

  function addElseBranch() {
    if ((command.branches ?? []).some((b) => b.default)) return;
    onChange({
      ...command,
      branches: [...(command.branches ?? []), { default: true, template: "" }],
    });
  }

  function removeBranch(branchIdx: number) {
    onChange({
      ...command,
      branches: (command.branches ?? []).filter((_, j) => j !== branchIdx),
    });
  }

  function moveBranch(branchIdx: number, dir: -1 | 1) {
    const branches = [...(command.branches ?? [])];
    const target = branchIdx + dir;
    if (target < 0 || target >= branches.length) return;
    [branches[branchIdx], branches[target]] = [
      branches[target],
      branches[branchIdx],
    ];
    onChange({ ...command, branches });
  }

  return (
    <div className="space-y-3 rounded border border-white/10 bg-black/20 p-3">
      <div className="flex gap-2">
        <input
          className={`${inputCls} w-40`}
          placeholder="Label"
          value={command.label}
          onChange={(e) => onChange({ ...command, label: e.target.value })}
        />
        <div className="flex rounded border border-white/15 text-xs">
          <button
            type="button"
            className={`px-3 py-1 ${
              !conditional
                ? "bg-white/10 text-white"
                : "opacity-60 hover:bg-white/5"
            }`}
            onClick={() => setMode("simple")}
          >
            Simple template
          </button>
          <button
            type="button"
            className={`px-3 py-1 ${
              conditional
                ? "bg-white/10 text-white"
                : "opacity-60 hover:bg-white/5"
            }`}
            onClick={() => setMode("conditional")}
          >
            Conditional branches
          </button>
        </div>
        <button
          className="ml-auto px-2 text-red-400 hover:text-red-300"
          onClick={onRemove}
        >
          ×
        </button>
      </div>

      {!conditional ? (
        <input
          className={`${inputCls} font-mono`}
          placeholder="Template: echo {{var}}"
          value={command.template ?? ""}
          onChange={(e) => onChange({ ...command, template: e.target.value })}
        />
      ) : (
        <div className="space-y-3">
          <p className="text-xs opacity-60">
            Branches are evaluated top-to-bottom. Unmatched commands are skipped at
            launch.
          </p>

          {(command.branches ?? []).map((branch, branchIdx) => (
            <BranchEditor
              key={branchIdx}
              branch={branch}
              branchIdx={branchIdx}
              branchCount={command.branches?.length ?? 0}
              namedVars={namedVars}
              testVars={testVars}
              inputCls={inputCls}
              onChange={(updated) => patchBranch(branchIdx, updated)}
              onRemove={() => removeBranch(branchIdx)}
              onMoveUp={() => moveBranch(branchIdx, -1)}
              onMoveDown={() => moveBranch(branchIdx, 1)}
            />
          ))}

          <div className="flex gap-3">
            <button
              type="button"
              className="text-xs text-[var(--accent)]"
              onClick={addBranch}
            >
              + Add branch
            </button>
            {!hasElse && (
              <button
                type="button"
                className="text-xs text-[var(--accent)]"
                onClick={addElseBranch}
              >
                + Add else branch
              </button>
            )}
          </div>

          {namedVars.length > 0 && (
            <div className="space-y-2 rounded border border-[var(--accent)]/30 bg-[var(--accent)]/5 p-3">
              <span className="text-xs font-medium uppercase opacity-70">
                Branch simulator
              </span>
              <div className="flex flex-wrap gap-2">
                {namedVars.map((v) => {
                  const val = testVars[v.name] ?? "";
                  const varMatch = testVariableRegex(v.regex, val);
                  return (
                    <div key={v.name} className="flex items-center gap-1">
                      <label className="text-xs opacity-60">{v.name}</label>
                      <input
                        className={`${inputCls} w-28 font-mono`}
                        placeholder="test value"
                        value={val}
                        onChange={(e) => onTestVarChange(v.name, e.target.value)}
                      />
                      {val !== "" && (
                        <span
                          className={`text-xs ${
                            varMatch.ok && !varMatch.error
                              ? "text-green-400"
                              : varMatch.error
                                ? "text-yellow-400"
                                : "text-red-400"
                          }`}
                          title={
                            varMatch.error ??
                            `Variable regex: ${v.regex || "(empty)"}`
                          }
                        >
                          {varMatch.ok && !varMatch.error
                            ? "valid"
                            : varMatch.error
                              ? "bad regex"
                              : "invalid"}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
              {resolution && (
                <p className="text-xs">
                  {resolution.error ? (
                    <span className="text-yellow-400">
                      Invalid regex — launch would fail: {resolution.error}
                    </span>
                  ) : resolution.ok ? (
                    <>
                      <span className="text-green-400">Matches: </span>
                      {resolution.branchIndex !== null
                        ? command.branches?.[resolution.branchIndex]?.default
                          ? "Else branch"
                          : `Branch ${resolution.branchIndex + 1}`
                        : "Legacy template"}
                      {" → "}
                      <code className="font-mono opacity-80">
                        {resolution.template || "(empty)"}
                      </code>
                    </>
                  ) : (
                    <span className="text-yellow-400">
                      No branch matches — command would be skipped at launch
                    </span>
                  )}
                </p>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
