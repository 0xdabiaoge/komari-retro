#!/usr/bin/env python3
"""Calculate the next Komari Retro product release from repository tags."""

from __future__ import annotations

import re
import subprocess
import sys
from collections.abc import Iterable


STABLE_TAG = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
MAX_COMPONENT = 99


class VersionError(ValueError):
    """Raised when release tags cannot produce a safe next version."""


def next_version(tags: Iterable[str]) -> str:
    versions: list[tuple[int, int, int]] = []
    for tag in tags:
        match = STABLE_TAG.fullmatch(tag.strip())
        if not match:
            continue

        version = tuple(int(component) for component in match.groups())
        if any(component < 1 or component > MAX_COMPONENT for component in version):
            raise VersionError(
                f"stable release tag {tag!r} has a component outside 1..{MAX_COMPONENT}"
            )
        versions.append(version)

    if not versions:
        raise VersionError("no stable vX.Y.Z release tags were found")

    major, minor, patch = max(versions)
    if patch < MAX_COMPONENT:
        patch += 1
    else:
        patch = 1
        if minor < MAX_COMPONENT:
            minor += 1
        else:
            minor = 1
            if major < MAX_COMPONENT:
                major += 1
            else:
                raise VersionError("version space exhausted at v99.99.99")

    return f"v{major}.{minor}.{patch}"


def repository_tags() -> list[str]:
    result = subprocess.run(
        ["git", "tag", "--list", "v*"],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.splitlines()


def main() -> int:
    try:
        print(next_version(repository_tags()))
    except (OSError, subprocess.CalledProcessError, VersionError) as error:
        print(f"Cannot generate the next release version: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
