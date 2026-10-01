import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import compatibility_observer as observer


class ObserverTests(unittest.TestCase):
    def setUp(self):
        self.modules = Path("/workspace/test/typescript/node_modules")
        self.rows = [
            dict(pid=100, ppid=10, rssKiB=100, cpu="0", name="time"),
            dict(pid=101, ppid=100, rssKiB=100, cpu="0", name="benchmark"),
            dict(pid=102, ppid=101, rssKiB=800000, cpu="100", name="tsc"),
            dict(pid=103, ppid=101, rssKiB=900000, cpu="100", name="tsgo"),
            dict(pid=200, ppid=10, rssKiB=2000000, cpu="100", name="tsc"),
            dict(pid=104, ppid=101, rssKiB=3000000, cpu="100", name="worker"),
            dict(pid=105, ppid=101, rssKiB=4000000, cpu="100", name="tsc"),
        ]

    def executable(self, pid):
        if pid == 105:
            return Path("/usr/local/bin/tsc")
        return self.modules / "@typescript/typescript-linux-x64/lib/tsc"

    def test_only_own_installed_checkers_are_eligible(self):
        candidates = observer.checker_candidates(self.rows, 100, self.modules, self.executable)
        self.assertEqual([row["pid"] for row in candidates], [103, 102])

    def test_exited_and_cyclic_processes_are_ignored(self):
        def exited(pid):
            raise ProcessLookupError()

        self.assertEqual(observer.checker_candidates(self.rows, 100, self.modules, exited), [])
        cycle = [dict(pid=102, ppid=103, rssKiB=800000, cpu="0", name="tsc"),
                 dict(pid=103, ppid=102, rssKiB=900000, cpu="0", name="tsc")]
        self.assertEqual(observer.checker_candidates(cycle, 100, self.modules, self.executable), [])

    def run_one_observation(self, available, free_swap):
        memory = dict(MemTotalKiB=16000000, MemAvailableKiB=available,
                      SwapTotalKiB=11000000, SwapFreeKiB=free_swap)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "progress.jsonl"
            with (mock.patch.object(observer, "process_rows", return_value=self.rows),
                  mock.patch.object(observer, "memory_values", return_value=memory),
                  mock.patch.object(observer, "checker_candidates", side_effect=lambda rows, root, modules, exe:
                                    observer_candidate(rows, root, modules, self.executable)),
                  mock.patch.object(observer.os, "kill") as kill,
                  mock.patch.object(observer.time, "sleep", side_effect=InterruptedError),
                  contextlib.redirect_stdout(io.StringIO())):
                with self.assertRaises(InterruptedError):
                    observer.observe(100, self.modules, output, 3072 * 1024)
                record = json.loads(output.read_text())
                return kill.call_args_list, record

    def test_memory_reserve_stops_only_owned_checker_and_records_reason(self):
        calls, row = self.run_one_observation(1000000, 1000000)
        self.assertEqual(calls, [mock.call(103, observer.signal.SIGKILL)])
        self.assertEqual(row["terminatedChecker"]["pid"], 103)
        self.assertIn("reserve", row["reason"])

    def test_available_swap_counts_toward_reserve(self):
        calls, row = self.run_one_observation(1000000, 5000000)
        self.assertEqual(calls, [])
        self.assertNotIn("terminatedChecker", row)


observer_candidate = observer.checker_candidates

if __name__ == "__main__":
    unittest.main()
