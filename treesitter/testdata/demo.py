#!/usr/bin/env python3
"""Drive micro-treesitter over stdin/stdout and print one reply.

Usage: demo.py [helper] [file] [filetype]
"""
import subprocess
import sys

helper = sys.argv[1] if len(sys.argv) > 1 else "./micro-treesitter"
path = sys.argv[2] if len(sys.argv) > 2 else "testdata/sample.go"
filetype = sys.argv[3] if len(sys.argv) > 3 else "go"

with open(path, "rb") as handle:
    text = handle.read()

process = subprocess.Popen(
    [helper, "--grammar-dir", "queries"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)
header = b"REQ\t1\t1\t" + filetype.encode() + b"\t" + str(len(text)).encode() + b"\n"
process.stdin.write(header + text)
process.stdin.flush()

reply = process.stdout.readline().decode()
sys.stdout.write(reply)
fields = reply.rstrip("\n").split("\t")
if fields[0] == "SPANS":
    for _ in range(int(fields[3])):
        sys.stdout.write(process.stdout.readline().decode())
elif fields[0] == "ERR":
    sys.stdout.write(process.stderr.read().decode())

process.stdin.close()
process.wait()
