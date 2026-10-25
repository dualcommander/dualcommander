"""Release-signing behavior checks; no certificates or network required."""
from pathlib import Path
import os
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class SigningTests(unittest.TestCase):
    def test_mac_commands(self):
        for failure in ('', 'reject', 'codesign'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / 'scripts').mkdir()
                shutil.copy(ROOT / 'scripts/sign-macos.sh', root / 'scripts/sign-macos.sh')
                (root / 'scripts/package-macos.sh').write_text('mkdir -p dist; touch dist/dualcommander-darwin-arm64.dmg\n')
                (root / 'scripts/package-macos_test.sh').write_text('exit 0\n')
                app = root / 'cmd/dualcommander/DualCommander.app'
                app.mkdir(parents=True)
                binary = root / 'bin'
                binary.mkdir()
                mock = binary / 'mock'
                mock.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
args = sys.argv[1:]
with open(os.environ['COMMAND_LOG'], 'a') as log:
    log.write(name + ' ' + ' '.join(args) + '\\n')
if name == 'go': print('arm64')
if name == 'openssl': print('temporary-password')
if name == 'codesign' and os.environ['FAILURE'] == 'codesign': sys.exit(1)
if name == 'ditto': Path(args[-1]).touch()
if name == 'xcrun' and args[0] == 'notarytool':
    print(json.dumps({'status': 'Invalid' if os.environ['FAILURE'] == 'reject' else 'Accepted'}))
''')
                mock.chmod(0o755)
                for command in ('go', 'security', 'openssl', 'codesign', 'ditto', 'xcrun', 'spctl'):
                    (binary / command).symlink_to(mock)
                env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'],
                           COMMAND_LOG=str(root / 'commands'), FAILURE=failure)
                for key in ('APPLE_CERTIFICATE_BASE64', 'APPLE_CERTIFICATE_PASSWORD',
                            'APPLE_SIGNING_IDENTITY', 'APPLE_API_KEY_BASE64',
                            'APPLE_API_KEY_ID', 'APPLE_API_ISSUER_ID'):
                    env[key] = 'dGVzdA=='
                result = subprocess.run(['sh', str(root / 'scripts/sign-macos.sh')], env=env,
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode == 0, not failure, result.stderr)
                log = (root / 'commands').read_text()
                self.assertIn('security delete-keychain', log)
                if failure:
                    self.assertNotIn('stapler staple', log)
                    self.assertFalse((root / 'dist/dualcommander-darwin-arm64.zip').exists())
                else:
                    self.assertEqual(log.count('notarytool submit'), 2)
                    self.assertEqual(log.count('stapler staple'), 2)
                    self.assertEqual(log.count('stapler validate'), 2)
                    self.assertTrue((root / 'dist/dualcommander-darwin-arm64.zip').exists())


if __name__ == '__main__':
    unittest.main()