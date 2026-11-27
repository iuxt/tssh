#!/usr/bin/env python3
"""Validate and synchronize the project version."""

import argparse
import os
from pathlib import Path
import re
import shutil
import stat
import sys

ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "VERSION"
MIRRORS = (ROOT / "tssh/VERSION", ROOT / "website/assets/VERSION")
VERSION_LINE = re.compile(rb"((?:0|[1-9][0-9]{0,4})(?:\.(?:0|[1-9][0-9]{0,4})){2})(\r?\n)?")
BUMP_INDEX = {"bump-major": 0, "bump-minor": 1, "bump-patch": 2}


def read_manifest(path):
    mode = path.lstat().st_mode
    if not stat.S_ISREG(mode):
        raise ValueError(f"{path.relative_to(ROOT)} must be a regular file")
    data = path.read_bytes()
    match = VERSION_LINE.fullmatch(data)
    if match is None or any(int(part) > 65534 for part in match[1].split(b".")):
        raise ValueError(f"{path.relative_to(ROOT)} must contain major.minor.patch (each part 0–65534)")
    return match[1], match[2] or b"", stat.S_IMODE(mode)


def get_version():
    return read_manifest(SOURCE)[0].decode("ascii")


def check_version():
    version = get_version()
    mismatches = []
    for path in MIRRORS:
        actual = read_manifest(path)[0].decode("ascii")
        if actual != version:
            mismatches.append(f"{path.relative_to(ROOT)} ({actual})")
    if mismatches:
        raise ValueError(f"VERSION is {version}; out of sync: {', '.join(mismatches)}. Run scripts/version.py sync.")
    return version


def write_manifests(changes, staging):
    staged = []
    for index, (path, old, new, mode) in enumerate(changes):
        backup = staging / f"{index}.old"
        replacement = staging / f"{index}.new"
        for temporary, data in ((backup, old), (replacement, new)):
            temporary.write_bytes(data)
            temporary.chmod(mode)
        staged.append((path, backup, replacement))

    replaced = []
    try:
        for path, backup, replacement in staged:
            os.replace(replacement, path)
            replaced.append((path, backup))
    except (OSError, KeyboardInterrupt):
        failures = []
        for path, backup in reversed(replaced):
            try:
                os.replace(backup, path)
            except OSError as error:
                failures.append(str(error))
        if failures:
            (staging / "recovery-required").touch()
            raise OSError(f"Rollback failed. Backups are in {staging}: {'; '.join(failures)}")
        raise


def update_version(command):
    staging = ROOT / ".version-update"
    try:
        staging.mkdir(mode=0o700)
    except FileExistsError:
        raise ValueError(f"Another version update is active or needs recovery in {staging}") from None

    try:
        manifests = [(path, *read_manifest(path)) for path in (SOURCE, *MIRRORS)]
        version = manifests[0][1]
        if command in BUMP_INDEX:
            index = BUMP_INDEX[command]
            parts = list(map(int, version.split(b".")))
            if parts[index] == 65534:
                raise ValueError(f"{command} would exceed 65534")
            parts[index] += 1
            parts[index + 1:] = [0] * (2 - index)
            version = ".".join(map(str, parts)).encode("ascii")

        changes = [
            (path, old + newline, version + newline, mode)
            for path, old, newline, mode in manifests
            if old != version
        ]
        write_manifests(changes, staging)
        return version.decode("ascii")
    finally:
        if not (staging / "recovery-required").exists():
            shutil.rmtree(staging)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("get", "check", "sync", *BUMP_INDEX))
    args = parser.parse_args()

    try:
        if args.command == "get":
            print(get_version())
        elif args.command == "check":
            print(check_version())
        else:
            print(update_version(args.command))
    except (OSError, ValueError) as error:
        parser.exit(1, f"version: {error}\n")


if __name__ == "__main__":
    main()
