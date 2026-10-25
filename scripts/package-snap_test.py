"""Snap version substitution checks; no snapd required."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class SnapTests(unittest.TestCase):
    def test_version_substitution(self):
        for arch in ('amd64', 'arm64'):
            for prefix in ('usr', 'usr/local'):
                with self.subTest(arch=arch, prefix=prefix), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    (root / 'scripts').mkdir()
                    (root / 'tools').mkdir()
                    (root / 'cmd/dualcommander').mkdir(parents=True)
                    shutil.copy(ROOT / 'scripts/package-snap.sh', root / 'scripts')
                    shutil.copytree(ROOT / 'snap', root / 'snap')
                    shutil.copy(ROOT / 'LICENSE', root)
                    (root / 'scripts/version.sh').write_text('echo 1.2.3+abcdef.dirty\n')
                    payload = root / 'payload/DualCommander' / prefix
                    (payload / 'bin').mkdir(parents=True)
                    (payload / 'bin/dualcommander').write_text('binary')
                    (payload / 'bin/dualcommander').chmod(0o755)
                    (payload / 'share/pixmaps').mkdir(parents=True)
                    (payload / 'share/pixmaps/io.github.typekpb.gommander.png').write_bytes(b'icon')
                    subprocess.run(['tar', '-cJf', str(root / 'cmd/dualcommander/DualCommander.tar.xz'),
                                    '-C', str(root / 'payload'), 'DualCommander'], check=True)
                    go = root / 'tools/go'
                    go.write_text(f'#!/bin/sh\ncase "$2" in GOOS) echo linux;; GOARCH) echo {arch};; esac\n')
                    go.chmod(0o755)
                    snapcraft = root / 'tools/snapcraft'
                    snapcraft.write_text('''#!/bin/sh
set -eu
grep -Fx "version: '1.2.3+abcdef.dirty'" snap/snapcraft.yaml
printf snap > "$3/dualcommander.snap"
''')
                    snapcraft.chmod(0o755)
                    env = dict(os.environ, PATH=str(root / 'tools') + os.pathsep + os.environ['PATH'])
                    subprocess.run(['sh', 'scripts/package-snap.sh'], cwd=root, env=env, check=True)


if __name__ == '__main__':
    unittest.main()