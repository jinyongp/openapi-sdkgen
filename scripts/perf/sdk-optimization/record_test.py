"""Behavior checks for failures that previously looked like passing evidence."""
import json
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import unittest

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


if __name__ == '__main__':
    unittest.main()
