#!/usr/bin/env python3
"""Export only Cline settings; never share live state or task history."""
import json
import os
from pathlib import Path
import sys
import tempfile


FILES = (
    "globalState.json",
    "secrets.json",
    "settings/providers.json",
    "settings/global-settings.json",
)


def copy_settings(source, target):
    # Validate everything before replacing any existing settings.
    settings = {}
    for name in FILES:
        path = source / name
        if path.is_file():
            raw = path.read_bytes()
            if not isinstance(json.loads(raw), dict):
                raise ValueError(f"Expected JSON object: {path}")
            settings[name] = raw
    if not settings:
        raise ValueError(f"No Cline settings found in {source}")
    target.mkdir(parents=True, exist_ok=True, mode=0o700)
    target.chmod(0o700)
    for name in FILES:
        path = target / name
        if name in settings:
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            path.write_bytes(settings[name])
            path.chmod(0o600)
        else:
            path.unlink(missing_ok=True)


def seed(source, target):
    copy_settings(source, target)
    mcp = target / "settings/cline_mcp_settings.json"
    mcp.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    mcp.write_text('{"mcpServers": {}}\n')
    mcp.chmod(0o600)


def self_test():
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        source, profile, target = (root / name for name in ("host", "profile", "container"))
        (source / "settings").mkdir(parents=True)
        for name in FILES:
            (source / name).write_text('{"test": "fixture"}')
        (source / "tasks").mkdir()
        (source / "tasks/history.json").write_text('{}')
        (source / "settings/cline_mcp_settings.json").write_text('{"mcpServers":{"host":{}}}')
        copy_settings(source, profile)
        seed(profile, target)
        for name in FILES:
            assert (target / name).read_bytes() == (source / name).read_bytes()
            assert (profile / name).stat().st_mode & 0o777 == 0o600
        assert profile.stat().st_mode & 0o777 == 0o700
        assert not (profile / "tasks").exists()
        assert not (profile / "settings/cline_mcp_settings.json").exists()
        assert json.loads((target / "settings/cline_mcp_settings.json").read_text()) == {"mcpServers": {}}
        (target / "globalState.json").write_text('{}')
        assert (profile / "globalState.json").read_text() != '{}'
        (source / "secrets.json").unlink()
        copy_settings(source, profile)
        seed(profile, target)
        assert not (target / "secrets.json").exists()
        (source / "globalState.json").write_text('[]')
        try:
            copy_settings(source, profile)
        except ValueError:
            pass
        else:
            raise AssertionError("Invalid settings accepted")
    print("Cline profile checks passed.")


if __name__ == "__main__":
    os.umask(0o077)
    try:
        if sys.argv[1:] == ["--self-test"]:
            self_test()
        elif sys.argv[1:] == ["--seed"]:
            seed(Path("/opt/cline-profile"), Path.home() / ".cline/data")
            print("Cline settings seeded; host history and MCP excluded.")
        elif not sys.argv[1:]:
            home = Path.home()
            source = Path(os.environ.get("CLINE_DATA_DIR") or
                          str(Path(os.environ.get("CLINE_DIR") or home / ".cline") / "data"))
            copy_settings(source, home / ".cline/devcontainer-profiles/dualcommander")
            print("Private Cline profile refreshed; rebuild container to apply.")
        else:
            sys.exit("Usage: cline-profile.py [--seed|--self-test]")
    except (OSError, ValueError):
        sys.exit("Cline profile failed: check settings JSON and filesystem permissions; no credentials logged.")