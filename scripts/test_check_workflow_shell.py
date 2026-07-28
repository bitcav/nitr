#!/usr/bin/env python3
"""Self-check for check_workflow_shell. Runs in CI before the real scan:
a guard that has never failed is not known to work, so every CI run proves
this one still catches the bug class it exists for.

Run: python3 scripts/test_check_workflow_shell.py
"""

import os
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import check_workflow_shell as c


def write(tmp, name, text):
    path = os.path.join(tmp, name)
    with open(path, "w") as fh:
        fh.write(text)
    return path


# The original bug: matrix resolves to windows, POSIX run, no shell anywhere.
BROKEN_MATRIX = """
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-2025]
    runs-on: ${{ matrix.os }}
    steps:
      - name: Check formatting
        run: test -z "$(gofmt -l .)"
"""

# Direct windows runs-on, no matrix involved.
BROKEN_DIRECT = """
jobs:
  win:
    runs-on: windows-2025
    steps:
      - run: echo hi
"""

# Same bug, fixed by declaring the shell on the step.
OK_STEP_SHELL = """
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-2025]
    runs-on: ${{ matrix.os }}
    steps:
      - name: Check formatting
        shell: bash
        run: test -z "$(gofmt -l .)"
"""

# Same bug, fixed by a job-level default (the fix master actually carries).
OK_JOB_DEFAULT = """
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-2025]
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        shell: bash
    steps:
      - name: Check formatting
        run: test -z "$(gofmt -l .)"
"""

# Implicit shell is fine where the job can never touch Windows.
OK_LINUX_ONLY = """
jobs:
  linux:
    runs-on: ubuntu-latest
    steps:
      - run: test -z "$(gofmt -l .)"
"""


def main():
    with tempfile.TemporaryDirectory() as tmp:
        failures = c.check_file(write(tmp, "broken.yml", BROKEN_MATRIX))
        assert len(failures) == 1, failures
        job, step, posix = failures[0]
        assert job == "test" and step == "Check formatting" and posix, failures

        failures = c.check_file(write(tmp, "direct.yml", BROKEN_DIRECT))
        assert len(failures) == 1 and failures[0][0] == "win", failures

        for name, text in (("step.yml", OK_STEP_SHELL),
                           ("default.yml", OK_JOB_DEFAULT),
                           ("linux.yml", OK_LINUX_ONLY)):
            assert c.check_file(write(tmp, name, text)) == [], name

        # Directory scanning finds all of them; only the two broken files fail.
        assert c.main(["check", tmp]) == 1

    print("test_check_workflow_shell: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
