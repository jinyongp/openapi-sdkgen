import contextlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import release_ci as ci


REPOSITORY = "owner/project"
SOURCE = "a" * 40


def run_record(**changes):
    record = dict(id=10, head_sha=SOURCE, event="push", head_branch="main",
                  path=".github/workflows/ci.yml", head_repository=dict(full_name=REPOSITORY),
                  status="completed", conclusion="success", html_url="https://github.com/owner/project/actions/runs/10")
    return record | changes


class CIReuseTests(unittest.TestCase):
    def test_requires_matching_source_workflow_repository_and_event(self):
        for changes in [dict(head_sha="b" * 40), dict(event="pull_request"),
                        dict(event="workflow_dispatch"), dict(head_branch="feature"),
                        dict(path=".github/workflows/other.yml"),
                        dict(head_repository=dict(full_name="fork/project")), dict(head_repository=None)]:
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                ci.latest_run([run_record(**changes)], REPOSITORY, SOURCE)

    def test_new_failed_run_cannot_be_hidden_by_old_success(self):
        with mock.patch.object(ci, "fetch_runs", return_value=[run_record(), run_record(id=11, conclusion="failure")]):
            with self.assertRaisesRegex(ValueError, "failure"):
                ci.require_ci(REPOSITORY, SOURCE)

    def test_waits_for_pending_ci_without_running_it_again(self):
        pending = run_record(status="in_progress", conclusion=None)
        with (mock.patch.object(ci, "fetch_runs", side_effect=[[pending], [run_record()]]),
              mock.patch.object(ci.time, "sleep") as sleep,
              contextlib.redirect_stderr(io.StringIO())):
            self.assertEqual(ci.require_ci(REPOSITORY, SOURCE)["id"], 10)
            sleep.assert_called_once_with(10)

    def test_rejects_missing_cancelled_or_skipped_ci(self):
        for records in [[], [run_record(conclusion="cancelled")], [run_record(conclusion="skipped")]]:
            with self.subTest(records=records), mock.patch.object(ci, "fetch_runs", return_value=records):
                with self.assertRaises(ValueError):
                    ci.require_ci(REPOSITORY, SOURCE)

    def test_pending_ci_has_a_bounded_wait(self):
        with (mock.patch.object(ci, "fetch_runs", return_value=[run_record(status="queued", conclusion=None)]),
              mock.patch.object(ci.time, "monotonic", side_effect=[0, 1201])):
            with self.assertRaises(TimeoutError):
                ci.require_ci(REPOSITORY, SOURCE)

    def test_new_run_started_while_waiting_supersedes_old_success(self):
        responses = [[run_record(status="queued", conclusion=None)],
                     [run_record(), run_record(id=11, conclusion="failure")]]
        with (mock.patch.object(ci, "fetch_runs", side_effect=responses),
              mock.patch.object(ci.time, "sleep"), contextlib.redirect_stderr(io.StringIO())):
            with self.assertRaises(ValueError):
                ci.require_ci(REPOSITORY, SOURCE)

    def test_reads_all_pages_and_requests_exact_source_ci(self):
        pages = [dict(workflow_runs=[run_record()]), dict(workflow_runs=[run_record(id=11)])]
        with mock.patch.object(ci.subprocess, "check_output", return_value=json.dumps(pages)) as command:
            self.assertEqual(len(ci.fetch_runs(REPOSITORY, SOURCE)), 2)
            args = command.call_args.args[0]
            self.assertIn("--paginate", args)
            self.assertIn("--slurp", args)
            self.assertIn("actions/workflows/ci.yml/runs?", args[-1])
            self.assertIn(f"head_sha={SOURCE}", args[-1])


class ReleaseMarkerTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.git("init", "-q")
        self.git("config", "user.name", "Release test")
        self.git("config", "user.email", "release@example.test")
        (self.root / "source").write_text("verified source")
        self.git("add", "source")
        self.git("-c", "commit.gpgsign=false", "commit", "-qm", "fix: source")
        self.source = self.git("rev-parse", "HEAD")

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args], text=True).strip()

    def marker(self, change=False):
        if change:
            (self.root / "source").write_text("unverified source")
            self.git("add", "source")
        self.git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm",
                 "chore(release): v1.2.2\n\nReleaseway-Config: " + "b" * 64)
        return self.git("rev-parse", "HEAD")

    def test_normal_source_requires_its_own_ci(self):
        with mock.patch.object(ci, "git", side_effect=self.git):
            self.assertEqual(ci.ci_source(self.source), self.source)

    def test_unchanged_marker_reuses_its_exact_parent(self):
        marker = self.marker()
        with mock.patch.object(ci, "git", side_effect=self.git):
            self.assertEqual(ci.ci_source(marker), self.source)

    def test_marker_with_changed_source_is_rejected(self):
        marker = self.marker(change=True)
        with mock.patch.object(ci, "git", side_effect=self.git):
            with self.assertRaises(ValueError):
                ci.ci_source(marker)

    def test_rejects_revision_expressions(self):
        with self.assertRaises(ValueError):
            ci.ci_source("HEAD~1")


if __name__ == "__main__":
    unittest.main()
