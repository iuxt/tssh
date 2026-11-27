#!/usr/bin/env python3
"""Build the native tssh tools, the Hugo website, or both."""

import argparse
from pathlib import Path
import subprocess

from version import ROOT, check_version


def build_binaries(output):
    goos = subprocess.check_output(["go", "env", "GOOS"], cwd=ROOT, text=True).strip()
    suffix = ".exe" if goos == "windows" else ""
    output.mkdir(parents=True, exist_ok=True)

    subprocess.run(["go", "build", "-o", str(output / f"tssh{suffix}"), "."], cwd=ROOT, check=True)
    transfer_module = ROOT / "third_party/trzsz-go"
    for name in ("trz", "tsz", "trzsz"):
        subprocess.run(
            ["go", "build", "-o", str(output / f"{name}{suffix}"), f"./cmd/{name}"],
            cwd=transfer_module,
            check=True,
        )


def build_website(output, base_url):
    command = ["hugo", "--source", "website", "--gc", "--minify", "--panicOnWarning",
               "--destination", str(output)]
    if base_url:
        command.extend(["--baseURL", base_url])
    subprocess.run(command, cwd=ROOT, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", nargs="?", default="binaries", choices=("binaries", "website", "all"))
    parser.add_argument("--output", type=Path, help="Output directory (for all, uses bin/ and website/ inside it)")
    parser.add_argument("--base-url", help="Override the site base URL")
    args = parser.parse_args()
    output = args.output.resolve() if args.output else None

    try:
        check_version()
        if args.target in ("binaries", "all"):
            binaries = output / "bin" if output and args.target == "all" else output
            build_binaries(binaries or ROOT / "dist/bin")
        if args.target in ("website", "all"):
            website = output / "website" if output and args.target == "all" else output
            build_website(website or ROOT / "website/public", args.base_url)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"build: {error}\n")


if __name__ == "__main__":
    main()
