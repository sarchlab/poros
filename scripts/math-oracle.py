#!/usr/bin/env python3
"""Generate/check the 15 guest-libm edge expectations without host libm.

Decimal ln/exp/sqrt use correctly rounded decimal arithmetic. Convert the exact
binary64 input to Decimal, independently evaluate at 100 and 200 digits, and
require identical final binary64 rounding. NaN payloads are outside this contract.
Run `python3 scripts/math-oracle.py --check` to verify the checked-in Go table.
"""
import argparse
import decimal
from pathlib import Path
import re
import struct

INPUTS = (
    0xbff0000000000000, 0x8000000000000000, 0, 1, 0x0010000000000000,
    0x3fe0000000000000, 0x3ff0000000000000, 0x3ff0000000000001,
    0x4000000000000000, 0x4085e00000000000, 0xc087480000000000,
    0x40862e42fefa39ef, 0x4086300000000000, 0x7ff0000000000000,
    0x7ff8000000001234,
)


def expected():
    rows = []
    for bits in INPUTS:
        value = struct.unpack(">d", struct.pack(">Q", bits))[0]
        exact = decimal.Decimal.from_float(value)
        results = []
        for operation in ("ln", "exp", "sqrt"):
            rounded = []
            for precision in (100, 200):
                with decimal.localcontext() as context:
                    context.prec = precision
                    context.traps[decimal.InvalidOperation] = False
                    context.traps[decimal.DivisionByZero] = False
                    result = float(getattr(exact, operation)())
                    rounded.append(struct.unpack(">Q", struct.pack(">d", result))[0])
            if rounded[0] != rounded[1]:
                raise ValueError(f"unstable rounding: {bits:x} {operation}")
            results.append(rounded[0])
        rows.append((bits, *results))
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    rows = expected()
    if args.check:
        source = Path(__file__).resolve().parents[1] / "pkg/elfsim/math_test.go"
        pattern = r"\{(0x[0-9a-f]+), \[3\]uint64\{(0x[0-9a-f]+), (0x[0-9a-f]+), (0x[0-9a-f]+)\}\}"
        checked = [tuple(int(x, 16) for x in row) for row in re.findall(pattern, source.read_text())]
        if checked != rows:
            raise SystemExit("guest math oracle table differs from high-precision expectations")
        print("45 guest math expectations verified at 100 and 200 decimal digits")
    else:
        for bits, *values in rows:
            print("{0x%016x, [3]uint64{%s}}," % (bits, ", ".join("0x%016x" % x for x in values)))


if __name__ == "__main__":
    main()
