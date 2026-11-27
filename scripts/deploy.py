#!/usr/bin/env python3
"""Trigger the GitHub release workflow by pushing the version tag."""

import argparse
import shlex
import subprocess

from version import ROOT, check_version


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--remote", default="origin", help="Git remote to receive the release tag")
    parser.add_argument("--dry-run", action="store_true", help="Print the tag push command without contacting the remote")
    args = parser.parse_args()

    try:
        version = check_version()
        commit = "HEAD"
        if not args.dry_run:
            commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
            status = subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True)
            if status:
                raise ValueError("Commit your changes before deploying")
        command = ["git", "push", "--no-follow-tags", "--", args.remote, f"{commit}:refs/tags/v{version}"]
        if args.dry_run:
            print(shlex.join(command))
        else:
            subprocess.run(command, cwd=ROOT, check=True)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"deploy: {error}\n")


if __name__ == "__main__":
    main()
