import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('upstream', Path(__file__).with_name('upstream-release.py'))
upstream = importlib.util.module_from_spec(spec)
spec.loader.exec_module(upstream)

class Releases(unittest.TestCase):
    def fixture(self, **changes):
        release = dict(tag_name='v2.6.0', draft=False, prerelease=False, published_at='2026-11-01T00:00:00Z')
        release.update(changes)
        return lambda path: {'sha': 'a' * 40} if path.startswith('commits/') else release

    def test_stable_release_is_bound_to_commit(self):
        self.assertEqual(upstream.resolve('', self.fixture()), dict(version='2.6.0', release_tag='v2.6.0', upstream_commit='a' * 40))

    def test_drafts_prereleases_and_unpublished_tags_are_refused(self):
        for change in [dict(draft=True), dict(prerelease=True), dict(published_at=None), dict(tag_name='v2.6.0-rc.1'), dict(tag_name='main')]:
            with self.subTest(change=change), self.assertRaises(ValueError):
                upstream.resolve('', self.fixture(**change))

    def test_manual_input_cannot_select_a_branch_or_expression(self):
        for tag in ['main', '../main', 'v2.6.0\nother=value', '$(id)', 'v2.6.0-beta']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                upstream.resolve(tag, self.fixture())

    def test_unresolved_commit_is_refused(self):
        with self.assertRaises(ValueError):
            upstream.resolve('', lambda path: {'sha':'main'} if path.startswith('commits/') else self.fixture()('release'))

if __name__ == '__main__':
    unittest.main()
