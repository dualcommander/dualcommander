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


if __name__ == '__main__':
    unittest.main()