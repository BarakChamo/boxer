#!/usr/bin/env python3
"""Time a command N times and report the distribution as JSON.

Wall clock from just before fork to just after reap, which is what an agent waiting on a tool
call actually experiences: process spawn, any client/daemon round trip, and the work itself.
Output is discarded so a chatty contender is not charged for the terminal.

Usage: timeit.py <reps> <warmups> -- <argv...>
"""
import json
import subprocess
import sys
import time

reps, warmups = int(sys.argv[1]), int(sys.argv[2])
argv = sys.argv[sys.argv.index("--") + 1:]

samples, failures = [], 0
for i in range(warmups + reps):
    t = time.monotonic()
    rc = subprocess.run(argv, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode
    d = (time.monotonic() - t) * 1000
    if i < warmups:
        continue
    if rc != 0:
        failures += 1
    samples.append(d)


def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(round((p / 100) * (len(xs) - 1))))]


print(json.dumps({
    "n": len(samples),
    "failures": failures,
    "min": round(min(samples), 1),
    "p50": round(pct(samples, 50), 1),
    "p90": round(pct(samples, 90), 1),
    "max": round(max(samples), 1),
}))
