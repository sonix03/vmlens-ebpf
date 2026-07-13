#!/usr/bin/env python3
"""Allocate memory gradually, capped at 256 MiB, then release it."""
import time

chunks = []
for used in range(16, 257, 16):
    chunks.append(bytearray(16 * 1024 * 1024))
    print(f"allocated={used}MiB", flush=True)
    time.sleep(1)
time.sleep(10)
