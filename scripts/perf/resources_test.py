"""Owned caches outlive their writers, but never outlive their measurement scope."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

from resources import go_cache


class ResourceTests(unittest.TestCase):
    def test_owned_cache_is_empty_and_reclaimed_on_exception(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = []
            for fail in (False, True):
                try:
                    with go_cache(directory, existing='' if fail else None) as cache:
                        path = Path(cache)
                        self.assertEqual(list(path.iterdir()), [])
                        (path / 'object').write_bytes(b'cold compilation')
                        paths.append(path)
                        if fail:
                            raise RuntimeError('compiler failed')
                except RuntimeError:
                    pass
                self.assertFalse(path.exists())
            self.assertEqual(len(set(paths)), 2)

    def test_borrowed_cache_and_off_are_not_owned(self):
        with tempfile.TemporaryDirectory() as directory:
            caller = Path(directory) / 'caller'
            caller.mkdir()
            sentinel = caller / 'keep'
            sentinel.write_bytes(b'owned by caller')
            for value in (str(caller), 'off'):
                with self.assertRaises(RuntimeError):
                    with go_cache(directory, existing=value) as actual:
                        self.assertEqual(actual, value)
                        raise RuntimeError('measurement failed')
            self.assertEqual(sentinel.read_bytes(), b'owned by caller')

    def test_cancelled_writer_tree_ends_before_cache_is_reclaimed(self):
        worker = ('from pathlib import Path;import sys,time\n'
                  'while True:Path(sys.argv[1]).write_text("object");time.sleep(.01)\n')
        writer = ('import subprocess,sys,time,json,os;from pathlib import Path;'
                  'p=subprocess.Popen([sys.executable,"-c",sys.argv[3],sys.argv[1]],start_new_session=True);'
                  'Path(sys.argv[2]).write_text(json.dumps({"root":os.getpid(),"child":p.pid,"cache":str(Path(sys.argv[1]).parent)}));'
                  'time.sleep(60)')
        controller = ('import json,sys;from pathlib import Path;'
                      'from resources import cancellation_signals,go_cache,owned_process\n'
                      'with cancellation_signals(),go_cache(sys.argv[1]) as cache:\n'
                      ' try:\n'
                      '  with owned_process([sys.executable,"-c",sys.argv[3],str(Path(cache)/"object"),sys.argv[2],sys.argv[5]]) as process:process.wait()\n'
                      ' finally:\n'
                      '  Path(sys.argv[4]).write_text(str(Path(cache).exists()))\n')
        for requested in (signal.SIGTERM, signal.SIGINT):
            with self.subTest(requested=requested), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                ready, before_cleanup = root / 'ready', root / 'before-cleanup'
                env = {**os.environ, 'PYTHONPATH': str(Path(__file__).parent)}
                process = subprocess.Popen([sys.executable, '-B', '-c', controller,
                    str(root), str(ready), writer, str(before_cleanup), worker], env=env,
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
                observed = None
                try:
                    deadline = time.monotonic() + 10
                    while not ready.exists():
                        self.assertIsNone(process.poll(), 'writer failed before becoming ready')
                        self.assertLess(time.monotonic(), deadline)
                        time.sleep(.02)
                    observed = json.loads(ready.read_text())
                    while not (Path(observed['cache']) / 'object').exists():
                        self.assertLess(time.monotonic(), deadline, 'nested writer never wrote cache data')
                        time.sleep(.01)
                    process.send_signal(requested)
                    _, stderr = process.communicate(timeout=20)
                    self.assertEqual(process.returncode, 128 + requested, stderr)
                    self.assertEqual(before_cleanup.read_text(), 'True')
                    self.assertFalse(Path(observed['cache']).exists())
                    for pid in (observed['root'], observed['child']):
                        status = Path('/proc') / str(pid) / 'stat'
                        if status.exists():
                            self.assertEqual(status.read_text().rsplit(')', 1)[1].split()[0], 'Z')
                finally:
                    if observed:
                        for pid in (observed['root'], observed['child']):
                            try:
                                os.kill(pid, signal.SIGKILL)
                            except ProcessLookupError:
                                pass
                    if process.poll() is None:
                        process.kill()
                    process.communicate(timeout=10)


if __name__ == '__main__':
    unittest.main()
