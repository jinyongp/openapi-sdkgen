import copy
import unittest

from releases import select_tags


def release(tag):
    return {"tag_name": tag, "draft": False, "immutable": True, "assets": [
        {"name": "install.sh", "digest": "sha256:" + "a" * 64},
        {"name": "install.ps1", "digest": "sha256:" + "b" * 64},
    ]}


class InstallerReleasesTest(unittest.TestCase):
    def test_retains_old_current_and_prerelease_installers(self):
        old = {"tag_name": "v0.1.0", "assets": []}
        draft = {**release("v2.0.0"), "draft": True}
        self.assertEqual(select_tags([
            release("v1.2.0"), old, draft, release("v1.1.0"), release("v1.3.0-rc.1"),
        ], "v1.2.0"), ["v1.1.0", "v1.2.0", "v1.3.0-rc.1"])

    def test_rejects_incomplete_or_unverifiable_history(self):
        for mutation in [
            lambda item: item["assets"].pop(),
            lambda item: item.update(immutable=False),
            lambda item: item["assets"][0].update(digest=None),
            lambda item: item.update(tag_name="../escape"),
        ]:
            with self.subTest(mutation=mutation):
                broken = copy.deepcopy(release("v1.0.0"))
                mutation(broken)
                with self.assertRaises(ValueError):
                    select_tags([broken, release("v1.1.0")], "v1.1.0")

    def test_requires_current_public_release_and_unique_tags(self):
        with self.assertRaises(ValueError):
            select_tags([release("v1.0.0")], "v1.1.0")
        with self.assertRaises(ValueError):
            select_tags([release("v1.0.0"), release("v1.0.0")], "v1.0.0")


if __name__ == "__main__":
    unittest.main()
