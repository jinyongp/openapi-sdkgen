import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

from compatibility_artifact import restore_corpus, select_artifact
from compatibility_matrix import corpus_matrix, document_matrix


class ArtifactTests(unittest.TestCase):
    def test_only_latest_matching_unexpired_artifact_is_used(self):
        older = dict(name="inputs", expired=False, created_at="2026-10-01T01:00:00Z", id=1)
        newest = dict(older, id=2, created_at="2026-10-01T02:00:00Z")
        expired = dict(newest, id=3, expired=True, created_at="2026-10-01T03:00:00Z")
        other = dict(expired, expired=False, name="different", id=4)
        self.assertEqual(select_artifact([other, newest, expired, older], "inputs"), newest)
        self.assertIsNone(select_artifact([expired, other], "inputs"))

    def test_manifest_change_invalidates_only_its_corpus_key(self):
        original = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifests = root / "test/compatibility"
            manifests.mkdir(parents=True)
            for name in ("holdout", "modern", "production32", "regression"):
                (manifests / f"{name}.json").write_bytes((original / "test/compatibility" / f"{name}.json").read_bytes())
            before = corpus_matrix(root)["include"]
            path = manifests / "modern.json"
            path.write_text(path.read_text() + "\n")
            after = corpus_matrix(root)["include"]
            self.assertEqual([a["corpus"] for a, b in zip(before, after) if a["artifact"] != b["artifact"]], ["modern"])
            keys = {job["corpus"]: job["artifact"] for job in after}
            self.assertTrue(all(job["artifact"] == keys[job["corpus"]] for job in document_matrix(root)["include"]))

    def test_restore_requires_validation_and_keeps_existing_destination(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / ".tmp/compatibility-restore"
            source.mkdir(parents=True)
            (source / "input.json").write_text("input")
            destination = root / ".tmp/compatibility-corpora/modern"
            with mock.patch("compatibility_artifact.subprocess.run", return_value=mock.Mock(returncode=1)):
                self.assertFalse(restore_corpus("modern", root))
            self.assertTrue(source.exists())
            self.assertFalse(destination.exists())
            with mock.patch("compatibility_artifact.subprocess.run", return_value=mock.Mock(returncode=0)):
                self.assertTrue(restore_corpus("modern", root))
                with self.assertRaises(RuntimeError):
                    restore_corpus("modern", root)
            self.assertFalse(source.exists())
            self.assertEqual((destination / "input.json").read_text(), "input")


if __name__ == "__main__":
    unittest.main()
