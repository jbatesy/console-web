import { RE2JS } from "re2js";
import type { Command, CommandBranch } from "./types";

export function isConditional(cmd: Command): boolean {
  return (cmd.branches?.length ?? 0) > 0;
}

export function resolveCommand(
  cmd: Command,
  vars: Record<string, string>,
): { template: string; ok: boolean; branchIndex: number | null; error?: string } {
  if (!cmd.branches?.length) {
    return {
      template: cmd.template ?? "",
      ok: Boolean(cmd.template),
      branchIndex: null,
    };
  }

  let defaultBranch: { branch: CommandBranch; index: number } | undefined;
  for (let i = 0; i < cmd.branches.length; i++) {
    const b = cmd.branches[i];
    if (b.default) {
      if (!defaultBranch) defaultBranch = { branch: b, index: i };
      continue;
    }
    if (!b.when || Object.keys(b.when).length === 0) continue;
    const result = matchesAll(b.when, vars);
    if (result.error) {
      return { template: "", ok: false, branchIndex: null, error: result.error };
    }
    if (result.matched) {
      return { template: b.template, ok: true, branchIndex: i };
    }
  }
  if (defaultBranch) {
    return {
      template: defaultBranch.branch.template,
      ok: true,
      branchIndex: defaultBranch.index,
    };
  }
  return { template: "", ok: false, branchIndex: null };
}

function fullMatchPattern(pattern: string): string {
  return `^(?:${pattern})$`;
}

function compileRE2(pattern: string): { re?: RE2JS; error?: string } {
  try {
    return { re: RE2JS.compile(pattern) };
  } catch (e) {
    return { error: e instanceof Error ? e.message : String(e) };
  }
}

function compileFullMatchRE2(pattern: string): { re?: RE2JS; error?: string } {
  return compileRE2(fullMatchPattern(pattern));
}

function matchesAll(
  when: Record<string, string>,
  vars: Record<string, string>,
): { matched: boolean; error?: string } {
  for (const [name, pattern] of Object.entries(when)) {
    const compiled = compileFullMatchRE2(pattern);
    if (compiled.error) {
      return {
        matched: false,
        error: `invalid regex ${JSON.stringify(pattern)} for ${JSON.stringify(name)}: ${compiled.error}`,
      };
    }
    if (!compiled.re!.test(vars[name] ?? "")) {
      return { matched: false };
    }
  }
  return { matched: true };
}

export function validateWhenPattern(
  pattern: string,
): { ok: boolean; error?: string } {
  if (!pattern) return { ok: false, error: "pattern is required" };
  const compiled = compileRE2(pattern);
  if (compiled.error) {
    return { ok: false, error: compiled.error };
  }
  return { ok: true };
}

export function testVariableRegex(
  regex: string,
  value: string,
): { ok: boolean; error?: string } {
  const compiled = compileFullMatchRE2(regex);
  if (compiled.error) {
    return { ok: false, error: compiled.error };
  }
  return { ok: compiled.re!.test(value) };
}
