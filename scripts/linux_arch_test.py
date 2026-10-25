"""Linux release architecture checks; no native packaging tools required."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class LinuxArchitectureTests(unittest.TestCase):
    def test_packager_accepts_native_architectures(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory)
            go = binary / 'go'
            go.write_text('#!/bin/sh\ncase "$2" in GOOS) echo linux;; GOARCH) echo "$TEST_ARCH";; esac\n')
            go.chmod(0o755)
            getconf = binary / 'getconf'
            getconf.write_text('#!/bin/sh\necho architecture-accepted >&2\nexit 73\n')
            getconf.chmod(0o755)
            for arch in ('amd64', 'arm64'):
                with self.subTest(arch=arch):
                    env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'], TEST_ARCH=arch)
                    result = subprocess.run(['sh', 'scripts/package-linux.sh'], cwd=ROOT,
                                            env=env, capture_output=True, text=True)
                    self.assertEqual(result.returncode, 73, result.stderr)
                    self.assertIn('architecture-accepted', result.stderr)

    def test_native_arm_release_job(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        self.assertRegex(workflow, r'runner: ubuntu-22\.04-arm\s+os: linux\s+arch: arm64')
        self.assertIn('dist/dualcommander-linux-${{ matrix.arch }}.tar.xz', workflow)
        self.assertIn('sh scripts/package-linux_test.sh', workflow)
        self.assertIn('gh release upload "$RELEASE_TAG" dist/*', workflow)

    def test_windows_arm_release_job(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        self.assertRegex(workflow, r'runner: windows-11-arm\s+os: windows\s+arch: arm64')
        self.assertIn('mingw-w64-clang-aarch64-clang', workflow)
        self.assertIn('CLANGARM64', workflow)
        self.assertIn('steps.msys2.outputs.msys2-location', workflow)
        self.assertIn('"CC=clang"', workflow)
        self.assertIn('"CXX=clang++"', workflow)
        self.assertIn('go test -tags ci -count=1 ./...', workflow)
        self.assertIn('go test -tags ci -race -count=1 ./...', workflow)
        self.assertIn('go env GOARCH', workflow)
        self.assertIn('dist/dualcommander-windows-${{ matrix.arch }}.zip', workflow)


if __name__ == '__main__':
    unittest.main()