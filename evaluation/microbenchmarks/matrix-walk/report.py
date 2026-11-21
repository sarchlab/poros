"""Render the three Apple matrix-walk profiles from raw Poros ELF results."""
import argparse
import csv
import json
import math
import sys
from pathlib import Path

SUITE = Path(__file__).resolve().parent
sys.path.insert(0, str(SUITE.parents[2] / "scripts"))
from accuracy_site import chart, esc, live_document


TARGETS = ("m1", "m2", "m5-max")
REFERENCE_IDS = ("m1-linux", "m2-linux", "m5-max-darwin")
REPEATS = 32


def read(name):
    return json.loads(name.read_text())


def interval(results, target, size, width, order):
    samples = []
    for count in (1, 1 + REPEATS):
        row = read(results / f"{target}-{size}-{width}-{order}-{count}.json")
        timing = row["timing"]
        if (row["args"][1:] != [order, str(size), str(width), str(count)]
                or row["termination"] != "exit" or row["exit_status"] != 0
                or row["stdout_base64"] != "UEFTUwo=" or row["stderr_base64"]
                or timing["target"]["name"] != target
                or row["build"]["vcs.modified"] != "false"):
            raise ValueError(f"Invalid matrix-walk result for {target}/{size}/{width}/{order}/{count}")
        samples.append((row, timing))

    (short, low), (long, high) = samples
    if (short["binary_sha256"] != long["binary_sha256"]
            or short["build"] != long["build"]
            or low["configuration"] != high["configuration"]
            or low["assumed_hz"] != high["assumed_hz"]
            or low["model"] != "ooo" or high["model"] != "ooo"):
        raise ValueError("A repetition pair used different binaries, builds or models")

    cycles = high["user_cycles"] - low["user_cycles"]
    instructions = high["retired_instructions"] - low["retired_instructions"]
    if cycles <= 0 or instructions <= 0 or low["assumed_hz"] <= 0:
        raise ValueError("A repetition pair has no positive timing or instruction delta")
    return {
        "revision": long["build"]["vcs.revision"],
        "binary_sha256": long["binary_sha256"],
        "frequency_hz": low["assumed_hz"],
        "cycles_per_walk": cycles / REPEATS,
        "instructions_per_walk": instructions / REPEATS,
        "us_per_walk": cycles / REPEATS / low["assumed_hz"] * 1e6,
    }


def collect(results, reference):
    configurations = [(r["size"], r["row_width"]) for r in reference["cases"]]
    references = {device["id"]: device for device in reference["references"]}
    rows = []
    identities = set()
    for target, reference_id in zip(TARGETS, REFERENCE_IDS, strict=True):
        device = references[reference_id]
        if target != device["target"]:
            raise ValueError("Reference and target differ")
        if [(r["size"], r["row_width"]) for r in device["cases"]] != configurations:
            raise ValueError("Reference cases must match the benchmark inventory in order")
        for native in device["cases"]:
            size, width = native["size"], native["row_width"]
            row, col = (interval(results, target, size, width, order) for order in ("r", "c"))
            for value in (row, col):
                identities.add((value["revision"], value["binary_sha256"]))
            if (row["frequency_hz"] != col["frequency_hz"]
                    or row["instructions_per_walk"] != col["instructions_per_walk"]):
                raise ValueError("Traversal orders changed frequency or repeated instruction count")

            native_ratio = native["paired_median_column_over_row"]
            simulated_ratio = col["cycles_per_walk"] / row["cycles_per_walk"]
            if not math.isfinite(native_ratio) or native_ratio <= 0:
                raise ValueError("Invalid native row/column ratio")
            rows.append({
                "target": target, "hardware": device["hardware"], "reference": reference_id,
                "size": size, "row_width": width, "paired_native_ratio": native_ratio,
                "poros_ratio": simulated_ratio,
                "ratio_error_percent": 100 * (simulated_ratio / native_ratio - 1),
                "native_row_us": native["median_row_ns"] / 1000,
                "native_column_us": native["median_column_ns"] / 1000,
                "poros_row_us": row["us_per_walk"],
                "poros_column_us": col["us_per_walk"],
                "poros_row_cycles_per_walk": row["cycles_per_walk"],
                "poros_column_cycles_per_walk": col["cycles_per_walk"],
                "poros_instructions_per_walk": row["instructions_per_walk"],
                "poros_frequency_hz": row["frequency_hz"],
            })
    if len(identities) != 1:
        raise ValueError("The report requires a single revision and guest executable")
    revision, binary_sha256 = identities.pop()
    return revision, binary_sha256, rows


def render(revision, binary_sha256, rows):
    headline = (
        '<header><h1>Matrix traversal</h1>'
        '<p>Apple M1, M2 and M5 Max · Column-first time divided by row-first time</p>'
        f'<p class="muted">Poros <code>{esc(revision[:12])}</code> · '
        '<a href="https://github.com/sarchlab/poros/issues/89">Issue #89</a> · '
        '<a href="pairs.csv">Download data</a> · <a href="reference.json">Native reference</a></p></header>'
    )
    y_top = max(5, 5 * math.ceil(max(max(r["paired_native_ratio"], r["poros_ratio"]) for r in rows) / 5))
    charts = []
    for target in TARGETS:
        cases = [r for r in rows if r["target"] == target]
        labels = [f'{r["size"]}/{r["row_width"]}' for r in cases]
        plots = []
        for title, key, color in (("Native", "paired_native_ratio", "#946414"),
                                  ("Poros", "poros_ratio", "#147c8f")):
            plots.append((title, [(i, r[key], f"{title} {labels[i]}: {r[key]:.2f}×")
                                  for i, r in enumerate(cases)], color))
        charts.append(chart(cases[0]["hardware"], plots, "Matrix size / row width", "Column ÷ row",
                            x_ticks=list(enumerate(labels)), y_ticks=[(j, f"{j}×") for j in range(0, y_top + 1, 5)],
                            y_bands=((0, 1, "#eef4f4"),),
                            note="Native · ochre     Poros · teal"))

    body = headline + '<div class="charts">' + "".join(charts) + '</div>'
    body += (
        '<h2>By device and input</h2><div class="table"><table><thead><tr>'
        '<th>Device</th><th>Size / row width</th><th>Native ratio</th><th>Poros ratio</th>'
        '<th>Ratio error</th><th>Native row / column (µs)</th><th>Poros row / column (µs)</th>'
        '</tr></thead><tbody>'
    )
    for r in rows:
        body += (
            f'<tr><td>{esc(r["hardware"])}</td><td>{r["size"]} / {r["row_width"]}</td>'
            f'<td>{r["paired_native_ratio"]:.2f}×</td><td>{r["poros_ratio"]:.2f}×</td>'
            f'<td>{r["ratio_error_percent"]:+.1f}%</td>'
            f'<td>{r["native_row_us"]:.2f} / {r["native_column_us"]:.2f}</td>'
            f'<td>{r["poros_row_us"]:.2f} / {r["poros_column_us"]:.2f}</td></tr>'
        )
    body += (
        '</tbody></table></div><h2>Measurement</h2>'
        '<p>Native ratios are medians of 82 adjacent row/column pairs, each measured over 32 warmed '
        'traversals. M1 and M2 use a fixed Linux performance core. M5 Max uses macOS user-initiated '
        'QoS with core placement unverified. The separate row and column medians in the table may '
        'divide to a slightly different ratio.</p>'
        '<p>Poros estimates the warmed cost by subtracting the one-traversal run from the 33-traversal '
        'run, then dividing by 32. Times use each modeled device’s configured clock. The simulator '
        'runs a Linux ELF. Native compiler, OS and memory placement can differ. Ratio error is '
        '(Poros ratio ÷ native ratio − 1) × 100%.</p>'
    )
    return live_document("Poros · Matrix traversal", body, {"revision": revision, "guest": binary_sha256}, False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("results", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    reference_file = SUITE / "reference.json"
    revision, binary_sha256, rows = collect(args.results, read(reference_file))
    args.output.mkdir(parents=True, exist_ok=True)
    with (args.output / "pairs.csv").open("w", newline="") as stream:
        writer = csv.DictWriter(stream, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    (args.output / "reference.json").write_bytes(reference_file.read_bytes())
    (args.output / "index.html").write_text(render(revision, binary_sha256, rows))


if __name__ == "__main__":
    main()
