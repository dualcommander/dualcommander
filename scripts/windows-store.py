"""Substitute release metadata into the versioned Store manifest."""
import os
from pathlib import Path
import re
from string import Template
import sys
from xml.sax.saxutils import escape


def package_version(tag):
    if not re.fullmatch(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', tag):
        raise ValueError('Store releases require vMAJOR.MINOR.PATCH tags')
    parts = [int(part) for part in tag[1:].split('.')]
    if not any(parts) or any(part > 65535 for part in parts):
        raise ValueError('Store version components must fit uint16; version must be nonzero')
    return tag[1:] + '.0'


def prepare_manifest(path, tag, arch, name, publisher, display):
    version = package_version(tag)
    if arch not in ('amd64', 'arm64') or not all(value.strip() for value in (name, publisher, display)):
        raise ValueError('Store architecture and identity metadata required')
    arch = 'x64' if arch == 'amd64' else arch
    template = Path(__file__).resolve().parent.parent / 'cmd/dualcommander/AppxManifest.xml'
    values = dict(name=name, publisher=publisher, display=display, version=version, arch=arch)
    path.write_text(Template(template.read_text(encoding='utf-8')).substitute(
        {key: escape(value, {'"': '&quot;'}) for key, value in values.items()}), encoding='utf-8')


if __name__ == '__main__':
    prepare_manifest(Path(sys.argv[1]), os.environ['RELEASE_TAG'], sys.argv[2],
                   os.environ['WINDOWS_STORE_IDENTITY_NAME'], os.environ['WINDOWS_STORE_PUBLISHER'],
                   os.environ['WINDOWS_STORE_PUBLISHER_DISPLAY_NAME'])