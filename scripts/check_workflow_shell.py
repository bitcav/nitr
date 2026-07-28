#!/usr/bin/env python3
"""Fail if a GitHub Actions `run:` step could execute on Windows without
declaring `shell:`.

THE BUG THIS EXISTS FOR (CI red on windows-2025, 2026-07-28):

    - name: Check formatting
      run: test -z "$(gofmt -l . | grep -v 'rice-box\\.go$')"

in a job with `runs-on: ${{ matrix.os }}` where the matrix included
windows-2025. On Windows the DEFAULT SHELL IS POWERSHELL, so `test`, the
`$(...)` idiom and `grep` are not what runs -- green on ubuntu, exit 1 on
Windows. The lesson was already written down three lines away (the smoke
test's explicit `shell: bash` comment) and was repeated anyway. A documented
lesson that does not execute does not prevent recurrence; this script is the
executable form.

WHY NOT actionlint: it does not catch this (verified 2026-07-28 against the
exact broken file -- clean exit 0). actionlint runs shellcheck on a `run:`
only when it can statically determine the shell, and `runs-on: ${{
matrix.os }}` defeats that inference.

THE RULE ENFORCED: in any job that could run on Windows -- `runs-on` naming
a windows image directly, or a matrix expression where any matrix value
mentions windows -- every step with `run:` must set `shell:` explicitly, or
the job must set `defaults.run.shell`. Declaring `bash` is fine (Git Bash
ships on GitHub's windows runners); so is `pwsh`. What is not fine is leaving
it implicit, because the implicit answer differs per OS and that is precisely
the trap.

Usage: check_workflow_shell.py [workflow-file-or-dir ...]
       defaults to .github/workflows in the current directory.
Exit: 0 clean, 1 on violations, unparseable workflow, or missing pyyaml.
"""

import os
import sys

# Deliberately NOT wrapped in try/except: in CI a missing dependency must
# hard-fail the job. Degrading to a skipped pass is the exact failure class
# this guard exists to prevent.
import yaml


def workflow_files(targets):
    for t in targets:
        if os.path.isdir(t):
            for root, _dirs, names in os.walk(t):
                for n in sorted(names):
                    if n.endswith((".yml", ".yaml")):
                        yield os.path.join(root, n)
        elif os.path.isfile(t):
            yield t


def mentions_windows(value):
    """True if any scalar anywhere inside value mentions windows."""
    if isinstance(value, str):
        return "windows" in value.lower()
    if isinstance(value, dict):
        return any(mentions_windows(v) for v in value.values())
    if isinstance(value, list):
        return any(mentions_windows(v) for v in value)
    return False


def job_may_run_on_windows(job):
    runs_on = job.get("runs-on")
    if runs_on is None:
        return False
    # Direct: runs-on: windows-2025  /  [windows-2025, ...]
    if mentions_windows(runs_on):
        return True
    # Templated: runs-on: ${{ matrix.os }} -- resolve through the matrix.
    if isinstance(runs_on, str) and "${{" in runs_on:
        matrix = (job.get("strategy") or {}).get("matrix")
        return matrix is not None and mentions_windows(matrix)
    return False


def uses_posix_idiom(run):
    """Heuristic: does this run: body rely on POSIX shell behaviour?

    Only used to ANNOTATE output. The rule enforced is the strict one
    (declare the shell), because a heuristic makes a poor gate -- it is
    exactly the kind of "looks fine to me" reasoning that let the original
    bug through.
    """
    idioms = ("$(", "`", "test -", "grep ", "[[", "&&", "||", " | ", "export ", "if [")
    return any(tok in run for tok in idioms)


def check_file(path):
    """Return [(job_name, step_name, posix_idiom), ...] for one workflow."""
    with open(path) as fh:
        doc = yaml.safe_load(fh)
    if not isinstance(doc, dict):
        return []
    failures = []
    for job_name, job in (doc.get("jobs") or {}).items():
        if not isinstance(job, dict) or not job_may_run_on_windows(job):
            continue
        # A job-level `defaults.run.shell` covers every step in the job.
        if ((job.get("defaults") or {}).get("run") or {}).get("shell"):
            continue
        for idx, step in enumerate(job.get("steps") or []):
            if not isinstance(step, dict) or "run" not in step:
                continue
            if step.get("shell"):
                continue
            name = step.get("name") or "step #%d" % (idx + 1)
            failures.append((job_name, name, uses_posix_idiom(step.get("run") or "")))
    return failures


def main(argv):
    targets = argv[1:] or [".github/workflows"]
    failures = []
    scanned = 0
    for path in workflow_files(targets):
        try:
            found = check_file(path)
        except Exception as exc:
            # An unparseable workflow must fail the guard, not be skipped --
            # silently passing is the failure class this exists to prevent.
            print("check_workflow_shell: FAIL -- could not parse %s: %s" % (path, exc))
            return 1
        scanned += 1
        failures.extend((path,) + f for f in found)

    if failures:
        print("check_workflow_shell: FAIL -- `run:` steps that may execute on Windows "
              "with no explicit `shell:`")
        print()
        for path, job, step, risky in failures:
            marker = "  <-- POSIX shell idiom: WILL misbehave under PowerShell" if risky else ""
            print("  %s  job=%s  step=%r%s" % (path, job, step, marker))
        print()
        if any(r for _, _, _, r in failures):
            print("  The marked step(s) are actively broken, not merely implicit.")
            print()
        print("  On Windows runners the default shell is PowerShell, so POSIX idioms")
        print("  (test, $(...), grep, pipes) silently do not run as written.")
        print("  Fix: add `shell: bash` to the step (Git Bash ships on windows runners),")
        print("  or set `defaults.run.shell` for the job.")
        return 1

    print("check_workflow_shell: ok (%d workflow file(s) scanned)" % scanned)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
