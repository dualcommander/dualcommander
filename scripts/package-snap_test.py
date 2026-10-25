"""Snap staging checks with mocked Snapcraft; no snapd required."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class SnapTests(unittest.TestCase):
    def test_packaging(self):
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
[ "$1" = pack ]
[ "$2" = --output ]
grep -Fx "version: '1.2.3+abcdef.dirty'" snap/snapcraft.yaml
grep -Fx 'confinement: classic' snap/snapcraft.yaml
test -x payload/bin/dualcommander
test -s payload/meta/gui/dualcommander.png
test -s payload/LICENSE
cmp "$TEST_DESKTOP" payload/meta/gui/dualcommander.desktop
grep -Fx 'Exec=dualcommander' payload/meta/gui/dualcommander.desktop
grep -Fx 'Icon=${SNAP}/meta/gui/dualcommander.png' payload/meta/gui/dualcommander.desktop
grep -Fx 'Categories=System;FileTools;FileManager;' payload/meta/gui/dualcommander.desktop
test -d "$3"
printf snap > "$3/dualcommander.snap"
''')
                    snapcraft.chmod(0o755)
                    env = dict(os.environ, PATH=str(root / 'tools') + os.pathsep + os.environ['PATH'],
                               TEST_DESKTOP=str(root / 'snap/gui/dualcommander.desktop'))
                    subprocess.run(['sh', 'scripts/package-snap.sh'], cwd=root, env=env, check=True)
                    self.assertEqual((root / f'dist/dualcommander-linux-{arch}.snap').read_text(), 'snap')

    def test_release_wiring(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        self.assertIn('make package-snap SNAPCRAFT_ARGS=--destructive-mode', workflow)
        self.assertIn('snap install snapcraft --classic', workflow)
        self.assertIn('python3 scripts/package-snap_test.py', (ROOT / 'Makefile').read_text())


if __name__ == '__main__':
    unittest.main()