#!/usr/bin/env python3
"""Resolve a stable upstream release and verify the source used by the image build."""
import argparse
import hashlib
import json
import os
import re
import tarfile
import urllib.parse
import urllib.request
from pathlib import Path

REPOSITORY = 'itteco/iframely'
VERSION = re.compile(r'^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')
COMMIT = re.compile(r'^[a-f0-9]{40}$')


def api(path):
    headers = {'Accept': 'application/vnd.github+json', 'User-Agent': 'IRCord-Iframely-Releases', 'X-GitHub-Api-Version': '2022-11-28'}
    token = os.environ.get('GH_TOKEN') or os.environ.get('GITHUB_TOKEN')
    if token:
        headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request('https://api.github.com/repos/' + REPOSITORY + '/' + path, headers=headers)
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def resolve(tag, get=api):
    if tag and not VERSION.fullmatch(tag):
        raise ValueError('Only stable vX.Y.Z or X.Y.Z release tags are accepted')
    release = get('releases/tags/' + urllib.parse.quote(tag, safe='') if tag else 'releases/latest')
    tag = release.get('tag_name', '')
    if release.get('draft') or release.get('prerelease') or not release.get('published_at') or not VERSION.fullmatch(tag):
        raise ValueError('Upstream release must be published, stable and versioned X.Y.Z')
    commit = get('commits/' + urllib.parse.quote(tag, safe='')).get('sha', '')
    if not COMMIT.fullmatch(commit):
        raise ValueError('Upstream release did not resolve to a full commit SHA')
    return {'version': tag.removeprefix('v'), 'release_tag': tag, 'upstream_commit': commit}


def source(commit, version, target):
    if not COMMIT.fullmatch(commit) or not VERSION.fullmatch(version):
        raise ValueError('Invalid source identity')
    target.parent.mkdir(parents=True, exist_ok=True)
    # No GitHub token is sent to the public source archive host.
    request = urllib.request.Request('https://codeload.github.com/' + REPOSITORY + '/tar.gz/' + commit, headers={'User-Agent': 'IRCord-Iframely-Releases'})
    with urllib.request.urlopen(request, timeout=60) as response, target.open('wb') as output:
        count = 0
        while chunk := response.read(65536):
            count += len(chunk)
            if count > 20 * 1024 * 1024:
                raise ValueError('Upstream source archive exceeds 20 MiB')
            output.write(chunk)
    with tarfile.open(target, 'r:gz') as archive:
        packages = [item for item in archive.getmembers() if item.name.count('/') == 1 and item.name.endswith('/package.json')]
        if len(packages) != 1 or packages[0].size > 65536:
            raise ValueError('Upstream archive has no bounded root package.json')
        package = json.load(archive.extractfile(packages[0]))
        if package.get('version') != version:
            raise ValueError('Release version and upstream package version differ')
    return {'source_sha256': hashlib.sha256(target.read_bytes()).hexdigest()}


def emit(values):
    for key, value in values.items():
        print(key + '=' + value)
    if output := os.environ.get('GITHUB_OUTPUT'):
        with open(output, 'a') as stream:
            for key, value in values.items():
                stream.write(key + '=' + value + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest='command', required=True)
    latest = commands.add_parser('resolve')
    latest.add_argument('--tag', default='')
    archive = commands.add_parser('source')
    archive.add_argument('--commit', required=True)
    archive.add_argument('--version', required=True)
    archive.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.command == 'resolve':
        emit(resolve(args.tag))
    else:
        emit(source(args.commit, args.version, args.output))
