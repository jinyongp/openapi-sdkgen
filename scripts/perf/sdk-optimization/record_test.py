"""Behavior checks for failures that previously looked like passing evidence."""
import json
import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent


class RecorderTest(unittest.TestCase):
    def trial(self, code, *flags):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root) / 'trial'
            command = [sys.executable, str(HERE / 'record.py'), '--output', str(output),
                       '--timeout', '10', *flags, '--', sys.executable, '-c', code]
            process = subprocess.run(command, capture_output=True, text=True, timeout=30)
            result = json.loads((output / 'result.json').read_text())
            return process, result, (output / 'stdout.log').read_text() if (output / 'stdout.log').exists() else ''

    def test_pass_and_executable_identity(self):
        process, result, _ = self.trial('print("complete")')
        self.assertEqual(process.returncode, 0, process.stderr)
        self.assertEqual(result['status'], 'pass')
        self.assertTrue(result['goVersion'].startswith('go version go1.27.2 '))
        self.assertEqual(result['environment']['GOTOOLCHAIN'], 'local')
        self.assertEqual(len(result['goSha256']), 64)
        self.assertIn(str(Path(sys.executable).resolve()), result['binaries'])

    def test_error_and_signal_are_failures(self):
        for code, expected, status in [('raise SystemExit(7)', 'fail', 7),
                                       ('import os,signal;os.kill(os.getpid(),signal.SIGTERM)', 'signal', -signal.SIGTERM)]:
            process, result, _ = self.trial(code)
            self.assertNotEqual(process.returncode, 0)
            self.assertEqual(result['status'], expected)
            self.assertEqual(result['exitCode'], status)

    def test_timeout_and_child_rss_are_not_passes(self):
        for code, flags, expected in [
            ('import time;time.sleep(10)', ('--timeout', '0.3'), 'timeout'),
            ('import subprocess,sys,time;subprocess.Popen([sys.executable,"-c","import time;x=bytearray(64*1024**2);time.sleep(10)"]);time.sleep(10)',
             ('--max-rss-mib', '48'), 'rss_protection'),
        ]:
            process, result, _ = self.trial(code, *flags)
            self.assertNotEqual(process.returncode, 0)
            self.assertEqual(result['status'], expected)

    def test_unavailable_command_is_nonzero_with_blocked_record(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root) / 'trial'
            process = subprocess.run([sys.executable, str(HERE / 'record.py'), '--output', str(output),
                                      '--', '/nonexistent-sdkgen-command'], capture_output=True, timeout=30)
            self.assertNotEqual(process.returncode, 0)
            self.assertEqual(json.loads((output / 'result.json').read_text())['status'], 'blocked')

    def test_timeout_stops_child_in_separate_process_group(self):
        code = 'import subprocess,sys,time;p=subprocess.Popen([sys.executable,"-c","import time;time.sleep(10)"],process_group=0);print(p.pid,flush=True);time.sleep(10)'
        process, result, output = self.trial(code, '--timeout', '0.5')
        self.assertNotEqual(process.returncode, 0)
        self.assertEqual(result['status'], 'timeout')
        status = Path('/proc') / output.strip() / 'stat'
        if status.exists():
            self.assertEqual(status.read_text().rsplit(')', 1)[1].split()[0], 'Z')

    def test_parent_cancellation_stops_owned_sessions_and_records_interruption(self):
        # Exercise a separate group and a nested trial's separate session.
        code = ('import os,subprocess,sys,time;from pathlib import Path;'
                'p=subprocess.Popen([sys.executable,"-c","import time;time.sleep(60)"],process_group=0);'
                'nested=subprocess.Popen([sys.executable,"-c","import time;time.sleep(60)"],start_new_session=True);'
                'print("probe "+os.environ["RECORDER_TEST_API_KEY"],flush=True);'
                'Path(sys.argv[1]).write_text(str(os.getpid())+" "+str(p.pid)+" "+str(nested.pid));time.sleep(60)')
        for requested, group in ((signal.SIGTERM, False), (signal.SIGINT, False),
                                 (signal.SIGTERM, True), (signal.SIGINT, True)):
            with self.subTest(requested=requested, group=group), tempfile.TemporaryDirectory() as root:
                output, ready = Path(root) / 'trial', Path(root) / 'ready'
                unrelated = subprocess.Popen([sys.executable, '-c', 'import time;time.sleep(60)'],
                                             start_new_session=True)
                process = subprocess.Popen([sys.executable, '-B', str(HERE / 'record.py'),
                    '--output', str(output), '--timeout', '30', '--', sys.executable, '-c', code, str(ready)],
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True,
                    env={**os.environ, 'RECORDER_TEST_API_KEY': 'trial-cancel-private-canary'})
                owned = []
                try:
                    deadline = time.monotonic() + 15
                    while time.monotonic() < deadline:
                        if ready.exists() and len(ready.read_text().split()) == 3:
                            owned = [int(value) for value in ready.read_text().split()]
                            break
                        if process.poll() is not None:
                            self.fail('recorder exited before the owned child was ready')
                        time.sleep(.02)
                    self.assertEqual(len(owned), 3, 'owned processes did not become ready')
                    if group:
                        os.killpg(process.pid, requested)
                    else:
                        process.send_signal(requested)
                    process.wait(timeout=15)
                    for pid in owned:
                        deadline = time.monotonic() + 3
                        while True:
                            try:
                                state = (Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[0]
                            except FileNotFoundError:
                                break
                            if state == 'Z':
                                break
                            self.assertLess(time.monotonic(), deadline,
                                            'owned process survived recorder cancellation')
                            time.sleep(.02)
                    self.assertIsNone(unrelated.poll(), 'cancelled another session')
                    result = json.loads((output / 'result.json').read_text())
                    self.assertEqual(result['status'], 'interrupted')
                    self.assertEqual(result['requestedSignal'], requested)
                    self.assertEqual(process.returncode, 128 + requested)
                    self.assertLess(result['exitCode'], 0)
                    log = (output / 'stdout.log').read_text()
                    self.assertIn('probe [REDACTED]', log)
                    self.assertNotIn('trial-cancel-private-canary', log)
                finally:
                    # Clean the reproducer even when checking the old broken version.
                    for pid in owned:
                        try:
                            os.kill(pid, signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                    if process.poll() is None:
                        process.kill()
                    process.communicate(timeout=10)
                    unrelated.terminate()
                    unrelated.wait(timeout=10)

    def test_observed_nested_orphan_keeps_ownership_without_its_controller(self):
        spec = importlib.util.spec_from_file_location('trial_recorder', HERE / 'record.py')
        recorder = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(recorder)
        owned = {}
        initial = {
            100: {'parent': 1, 'session': 100, 'start': '10'},
            101: {'parent': 100, 'session': 100, 'start': '11'},
            102: {'parent': 101, 'session': 102, 'start': '12'},
            200: {'parent': 1, 'session': 200, 'start': '20'},
        }
        recorder.track_owned(initial, 100, owned)
        self.assertEqual(set(owned), {100, 101, 102})
        orphaned = {
            102: {'parent': 1, 'session': 102, 'start': '12'},
            103: {'parent': 102, 'session': 103, 'start': '13'},
            200: initial[200],
        }
        recorder.track_owned(orphaned, 100, owned)
        self.assertEqual(set(owned), {102, 103})
        recorder.track_owned({200: initial[200]}, 100, owned)
        self.assertEqual(owned, {})

    def test_worker_spawned_after_snapshot_is_recovered_before_controller_dies(self):
        spec = importlib.util.spec_from_file_location('trial_recorder', HERE / 'record.py')
        recorder = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(recorder)
        code = ('import signal,subprocess,sys,time;from pathlib import Path\n'
                'def spawn(number,frame):\n'
                ' p=subprocess.Popen([sys.executable,"-c","import time;time.sleep(60)"],start_new_session=True)\n'
                ' Path(sys.argv[2]).write_text(str(p.pid))\n'
                'signal.signal(signal.SIGUSR1,spawn)\n'
                'Path(sys.argv[1]).write_text("ready")\n'
                'while True:time.sleep(.01)\n')
        with tempfile.TemporaryDirectory() as root:
            ready, child_file = Path(root) / 'ready', Path(root) / 'child'
            process = subprocess.Popen([sys.executable, '-c', code, str(ready), str(child_file)],
                                       start_new_session=True)
            child = None
            try:
                deadline = time.monotonic() + 5
                while not ready.exists():
                    self.assertLess(time.monotonic(), deadline)
                    time.sleep(.01)
                stale = recorder.process_snapshot()
                process.send_signal(signal.SIGUSR1)
                while not child_file.exists() or not child_file.read_text():
                    self.assertLess(time.monotonic(), deadline)
                    time.sleep(.01)
                child = int(child_file.read_text())
                snapshot = recorder.process_snapshot
                initial = [True]
                def concurrent_snapshot():
                    if initial:
                        initial.clear()
                        return stale
                    return snapshot()
                with mock.patch.object(recorder, 'process_snapshot', side_effect=concurrent_snapshot):
                    recorder.stop(process)
                deadline = time.monotonic() + 3
                while True:
                    try:
                        state = (Path('/proc') / str(child) / 'stat').read_text().rsplit(')', 1)[1].split()[0]
                    except FileNotFoundError:
                        break
                    if state == 'Z':
                        break
                    self.assertLess(time.monotonic(), deadline, 'new-session worker escaped cleanup')
                    time.sleep(.02)
            finally:
                if child is not None:
                    try:
                        os.kill(child, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                if process.poll() is None:
                    process.kill()
                process.wait(timeout=5)


if __name__ == '__main__':
    unittest.main()
