"""Private resource lifetimes shared by optional local performance drivers."""
from contextlib import contextmanager
from functools import cache
import importlib.util
from pathlib import Path
import signal
import subprocess
import tempfile

POLICY_PATH = Path(__file__).resolve()


@contextmanager
def go_cache(parent, *, existing=None, prefix='go-cache-'):
    if existing is not None and str(existing):
        # Caller-owned paths (and GOCACHE=off) are never created or removed here.
        yield str(existing)
        return
    with tempfile.TemporaryDirectory(prefix=prefix, dir=parent) as directory:
        yield directory


@contextmanager
def cancellation_signals():
    numbers = (signal.SIGINT, signal.SIGTERM)
    previous = {number: signal.getsignal(number) for number in numbers}
    def cancel(number, frame):
        # Let the first request complete child shutdown and cache finalizers.
        for item in numbers:
            signal.signal(item, signal.SIG_IGN)
        raise SystemExit(128 + number)
    try:
        for number in numbers:
            signal.signal(number, cancel)
        yield
    finally:
        for number, handler in previous.items():
            signal.signal(number, handler)


@cache
def recorder():
    source = POLICY_PATH.parent / 'sdk-optimization/record.py'
    spec = importlib.util.spec_from_file_location('performance_trial_recorder', source)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def finish_process(process):
    observer = recorder()
    owned = {}
    observer.session_usage(process.pid, owned)
    try:
        if process.poll() is None:
            try:
                process.terminate()
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                pass
    finally:
        observer.stop(process, owned)


@contextmanager
def owned_process(command, **kwargs):
    process = subprocess.Popen(command, start_new_session=True, **kwargs)
    try:
        yield process
    finally:
        finish_process(process)
