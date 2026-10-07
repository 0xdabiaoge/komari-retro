import importlib.util
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("next_release_version.py")
SPEC = importlib.util.spec_from_file_location("next_release_version", MODULE_PATH)
assert SPEC and SPEC.loader
module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(module)


class NextReleaseVersionTests(unittest.TestCase):
    def test_increments_patch_and_uses_highest_stable_tag(self):
        self.assertEqual(
            module.next_version(["v1.2.5", "v1.2.6", "v1.2.7-rc.1"]),
            "v1.2.7",
        )

    def test_patch_rolls_over_to_one_and_carries_minor(self):
        self.assertEqual(module.next_version(["v1.2.99"]), "v1.3.1")

    def test_minor_rolls_over_and_carries_major(self):
        self.assertEqual(module.next_version(["v1.99.99"]), "v2.1.1")

    def test_rejects_out_of_range_stable_component(self):
        with self.assertRaises(module.VersionError):
            module.next_version(["v1.2.100"])

    def test_rejects_missing_stable_tags(self):
        with self.assertRaises(module.VersionError):
            module.next_version(["v1.2.7-rc.1"])

    def test_rejects_exhausted_version_space(self):
        with self.assertRaises(module.VersionError):
            module.next_version(["v99.99.99"])


if __name__ == "__main__":
    unittest.main()
