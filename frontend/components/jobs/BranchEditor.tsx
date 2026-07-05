"use client";

import { validateWhenPattern, testVariableRegex } from "@/lib/branches";
import type { CommandBranch, Variable } from "@/lib/types";

type BranchEditorProps = {
  branch: CommandBranch;
  branchIdx: number;
  branchCount: number;
  namedVars: Variable[];
  testVars: Record<string, string>;
  inputCls: string;
  onChange: (branch: CommandBranch) => void;
  onRemove: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
};

export function BranchEditor({
  branch,
  branchIdx,
  branchCount,
  namedVars,
  testVars,
  inputCls,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
}: BranchEditorProps) {
  function patchWhen(varName: string, regex: string) {
    if (branch.default) return;
    const when = { ...(branch.when ?? {}) };
    if (varName) when[varName] = regex;
    onChange({ ...branch, when });
  }

  function addWhenRow() {
    if (branch.default) return;
    const used = new Set(Object.keys(branch.when ?? {}));
    const next = namedVars.find((v) => v.name && !used.has(v.name));
    if (!next) return;
    patchWhen(next.name, "");
  }

  function removeWhenRow(varName: string) {
    if (branch.default) return;
    const when = { ...(branch.when ?? {}) };
    delete when[varName];
    onChange({ ...branch, when });
  }

  function changeWhenVar(oldName: string, newName: string) {
    if (branch.default) return;
    const when = { ...(branch.when ?? {}) };
    const regex = when[oldName] ?? "";
    delete when[oldName];
    if (newName) when[newName] = regex;
    onChange({ ...branch, when });
  }

  const usedCount = Object.keys(branch.when ?? {}).length;
  const noVars = namedVars.length === 0;
  const allUsed = !noVars && usedCount >= namedVars.length;

  return (
    <div className="space-y-2 rounded border border-white/10 bg-black/30 p-3">
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium uppercase opacity-70">
          {branch.default ? "Else" : `Branch ${branchIdx + 1}`}
        </span>
        <div className="ml-auto flex gap-1">
          {!branch.default && (
            <>
              <button
                type="button"
                className="px-1 text-xs opacity-60 hover:opacity-100"
                disabled={branchIdx === 0}
                onClick={onMoveUp}
              >
                ↑
              </button>
              <button
                type="button"
                className="px-1 text-xs opacity-60 hover:opacity-100"
                disabled={branchIdx >= branchCount - 1}
                onClick={onMoveDown}
              >
                ↓
              </button>
            </>
          )}
          <button
            type="button"
            className="px-1 text-xs text-red-400 hover:text-red-300"
            onClick={onRemove}
          >
            Remove
          </button>
        </div>
      </div>

      {!branch.default && (
        <div className="space-y-1">
          {Object.entries(branch.when ?? {}).map(([varName, regex]) => {
            const pattern = validateWhenPattern(regex);
            const testVal = testVars[varName];
            const match =
              pattern.ok && testVal !== undefined
                ? testVariableRegex(regex, testVal)
                : null;
            const selectableVars = namedVars.filter(
              (v) => v.name === varName || !Object.hasOwn(branch.when ?? {}, v.name),
            );
            return (
              <div key={varName} className="space-y-0.5">
                <div className="flex gap-2">
                  <select
                    className={`${inputCls} w-32`}
                    value={varName}
                    onChange={(e) => changeWhenVar(varName, e.target.value)}
                  >
                    {selectableVars.map((v) => (
                      <option key={v.name} value={v.name}>
                        {v.name}
                      </option>
                    ))}
                  </select>
                  <input
                    className={`${inputCls} font-mono${
                      pattern.error ? " border-yellow-500/50" : ""
                    }`}
                    placeholder="^regex$"
                    value={regex}
                    onChange={(e) => patchWhen(varName, e.target.value)}
                  />
                  {match && (
                    <span
                      className={`shrink-0 self-center text-xs ${
                        match.ok ? "text-green-400" : "text-red-400"
                      }`}
                      title="Match against simulator value"
                    >
                      {match.ok ? "✓" : "✗"}
                    </span>
                  )}
                  <button
                    type="button"
                    className="px-2 text-red-400 hover:text-red-300"
                    onClick={() => removeWhenRow(varName)}
                  >
                    ×
                  </button>
                </div>
                {pattern.error && (
                  <p className="text-xs text-yellow-400/80">{pattern.error}</p>
                )}
              </div>
            );
          })}
          <div className="space-y-1">
            <button
              type="button"
              className="text-xs text-[var(--accent)] disabled:opacity-40"
              disabled={noVars || allUsed}
              onClick={addWhenRow}
            >
              + Add condition
            </button>
            {noVars && (
              <p className="text-xs text-yellow-400/80">
                Define a variable in the Variables section below to add conditions.
              </p>
            )}
            {allUsed && (
              <p className="text-xs opacity-60">
                All variables are used. Each condition must use a distinct variable
                (all conditions must match).
              </p>
            )}
            {!noVars && usedCount === 0 && (
              <p className="text-xs text-yellow-400/80">
                Non-default branches need at least one condition.
              </p>
            )}
          </div>
        </div>
      )}

      <input
        className={`${inputCls} font-mono`}
        placeholder="Template: echo {{var}}"
        value={branch.template}
        onChange={(e) => onChange({ ...branch, template: e.target.value })}
      />
    </div>
  );
}
