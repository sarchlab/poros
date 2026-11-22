"""Render a device and benchmark dashboard from a single JSON file."""

import argparse
import json
import math
from pathlib import Path

from accuracy_site import POINT_TIPS, VIEW_SCRIPT, accuracy_charts, esc, live_document, page, view_style


STYLES = """<style>
.dashboard-card{margin:24px 0;padding:18px;border:1px solid #d7e1e4;border-radius:9px;background:white}
.dashboard-card h3{margin:0 0 14px}.dashboard-card h3 .badge{margin-left:10px}
.dashboard-card .charts{grid-template-columns:repeat(3,minmax(0,1fr))}
.dashboard-card figure{background:transparent;border:0;border-radius:0;padding:0;margin:6px 0 12px}
.dashboard-chart-tools,.dashboard-global{display:flex;align-items:center;flex-wrap:wrap}
.dashboard-card .dashboard-chart{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;column-gap:10px}
.dashboard-chart figcaption{margin:0;overflow-wrap:anywhere}
.dashboard-chart svg,.dashboard-chart > .muted{grid-column:1/-1}
.dashboard-chart svg{display:block;min-width:0;margin-top:8px}
.dashboard-chart-tools{gap:7px}
.dashboard-global{gap:8px 16px;margin:12px 0 18px}
.dashboard-global .muted{margin:0}
.dashboard-scale{display:inline-flex;align-items:center;gap:6px;color:#526a74;font-size:.8rem;white-space:nowrap}
.dashboard-switch{display:inline-flex;padding:2px;gap:2px;background:#eaf1f2;border-radius:7px}
.dashboard-switch button,.dashboard-reset{font:inherit;cursor:pointer}
.dashboard-switch button{min-height:25px;border:0;border-radius:5px;padding:2px 8px;color:#385762;background:transparent}
.dashboard-switch button[aria-pressed=true]{background:#1d424d;color:#fff}
.dashboard-reset{font-size:.8rem;color:#285966;border:0;border-radius:4px;background:transparent;padding:4px}
.dashboard-reset:disabled{visibility:hidden}
.dashboard-switch button:focus-visible,.dashboard-reset:focus-visible{outline:2px solid #087e8b;outline-offset:2px}
.dashboard-chart svg{touch-action:pan-y}.dashboard-chart svg circle{cursor:crosshair}
.dashboard-chart .muted{margin:0 0 8px}
.dashboard-configuration{display:flex;align-items:center;gap:8px;margin:0 0 16px}
.dashboard-configuration select{font:inherit;min-width:0;max-width:100%;padding:5px 8px;border:1px solid #bacbd0;border-radius:6px;background:white;color:inherit}
.dashboard-card .dashboard-panels .charts{grid-template-columns:repeat(2,minmax(0,1fr))}
.dashboard-summary-charts{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,300px),1fr));gap:16px}
.dashboard-summary-charts .charts{display:contents}
.dashboard-summary-charts figure{min-width:0}
.dashboard-devices{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,190px),1fr));gap:12px}
.dashboard-devices a{display:grid;gap:4px;padding:12px;border:1px solid #d7e1e4;border-radius:8px;background:white}
.dashboard-devices small{color:#526a74}
.dashboard-links{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0 24px}
.dashboard-links a{background:white;border:1px solid #d7e1e4;border-radius:6px;padding:4px 8px}
@media(max-width:980px){.dashboard-card .charts{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:640px){
  .dashboard-card .charts{grid-template-columns:minmax(0,1fr);gap:14px}
  .dashboard-card{padding:12px}
  .dashboard-card .dashboard-panels .charts{grid-template-columns:minmax(0,1fr)}
  .dashboard-configuration{display:grid;grid-template-columns:minmax(0,1fr);gap:5px}
  .dashboard-configuration select{box-sizing:border-box;width:100%;min-height:40px;font-size:16px}
  .dashboard-chart-tools{justify-content:flex-end}
  .dashboard-chart figcaption{font-size:.95rem}
  .dashboard-reset:disabled{display:none}
  .dashboard-switch button{min-height:34px}
  .dashboard-card figure{margin:0}
  .dashboard-chart .muted{margin:0}
}
@media(pointer:coarse){.dashboard-wheel-hint{display:none}}
</style>"""

KINDS = {"full": "Full benchmark", "microbenchmark": "Microbenchmark"}
MILLISECONDS_PER_UNIT = {"ns": 1e-6, "µs": 1e-3, "μs": 1e-3, "us": 1e-3, "ms": 1, "s": 1000}
MEASUREMENTS = (("native", "Hardware", "#9a6a17"), ("poros", "Poros", "#087e8b"))


def text(value, label):
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{label} must be a nonempty string")
    return value


def number(value, label):
    if type(value) not in (int, float) or not math.isfinite(value):
        raise ValueError(f"{label} must be a finite number")
    return value


def group(data):
    """Index exactly the devices and series present in the observations."""
    devices = {}
    seen = set()
    for benchmark in data["benchmarks"]:
        key = text(benchmark["id"], "Benchmark ID")
        if key in seen or benchmark["kind"] not in KINDS:
            raise ValueError(f"Duplicate benchmark or unknown kind: {key}")
        seen.add(key)
        for field in ("name", "scaling_factor", "time_unit"):
            text(benchmark[field], field)
        if benchmark["time_unit"] not in MILLISECONDS_PER_UNIT:
            raise ValueError(f'Unsupported summary timing unit: {benchmark["time_unit"]}')
        for point in benchmark["points"]:
            device = text(point["device"], "Device")
            scale = number(point["scale"], "Scaling value")
            factors = point.get("series", {})
            if not isinstance(factors, dict) or any(type(v) not in (str, int, float, bool) for v in factors.values()):
                raise ValueError("Series factors must be named scalar values")
            series_id = json.dumps(factors, sort_keys=True, allow_nan=False)
            for field, _, _ in MEASUREMENTS:
                value = point.get(field)
                if value is not None and number(value, field) < 0:
                    raise ValueError(f"{field} timing cannot be negative")
            card = devices.setdefault(device, {}).setdefault(key, (benchmark, {}))
            points = card[1].setdefault(series_id, {})
            if scale in points:
                raise ValueError(f"Duplicate scale {scale} for {device}/{key}/{series_id}")
            points[scale] = point
    return devices


def factor_label(factors):
    return " · ".join(f"{name}: {value}" for name, value in sorted(factors.items()))


def summary_points(cards, device_label):
    for benchmark, panels in cards:
        for key, points in panels.items():
            factors = factor_label(json.loads(key))
            for scale, point in points.items():
                native, poros = point.get("native"), point.get("poros")
                if not native or not poros or point.get("reference_quality", "usable") != "usable":
                    continue
                unit = benchmark["time_unit"]
                multiplier = MILLISECONDS_PER_UNIT[unit]
                point_label = point.get("label") or f'{benchmark["scaling_factor"]} {scale:g}'
                label = " · ".join(part for part in
                                   (device_label, benchmark["name"], factors, point_label) if part)
                yield benchmark["kind"], (native * multiplier, poros * multiplier, label)


def summary(points, heading):
    micro_count = sum(kind == "microbenchmark" for kind, _ in points)
    full_count = len(points) - micro_count
    return (
        f'<h2>{esc(heading)}</h2><p class="muted">'
        f'{full_count} full benchmark points · {micro_count} microbenchmark points. '
        'Each paired point counts once.</p><div class="dashboard-summary-charts">'
        + accuracy_charts([point for _, point in points], time_label="time") + "</div>"
    )


def y_switch(scope, label):
    title = "All charts · Y scale" if scope == "global" else "Y"
    return (
        f'<div class="dashboard-scale"><span aria-hidden="true">{title}</span>'
        f'<div class="dashboard-switch" role="group" aria-label="{esc(label)}">'
        f'<button type="button" data-{scope}-y="linear" aria-pressed="true">Linear</button>'
        f'<button type="button" data-{scope}-y="log" aria-pressed="false">Log</button></div></div>'
    )


def panel(factors, points, benchmark, chart_id):
    title = factor_label(factors) or benchmark["name"]
    dataset = {
        "xLabel": benchmark["scaling_factor"], "unit": benchmark["time_unit"],
        "series": MEASUREMENTS,
        "points": [
            [x, row.get("native"), row.get("poros"),
             row.get("label") or f'{benchmark["scaling_factor"]} {x:g}',
             row.get("reference_quality", "usable")]
            for x, row in sorted(points.items())
        ],
    }
    return (
        f'<figure id="{chart_id}" class="dashboard-chart" data-chart="{esc(json.dumps(dataset))}">'
        f'<figcaption>{esc(title)}</figcaption>'
        '<div class="dashboard-chart-tools">'
        '<button class="dashboard-reset" type="button" data-reset-x disabled>Reset X</button>'
        + y_switch("chart", f"Y scale for {title}") + '</div>'
        f'<svg viewBox="0 0 440 290" role="group" aria-label="{esc(title)}"></svg>'
        '<p class="muted"><span style="color:#9a6a17">Hardware</span> · '
        '<span style="color:#087e8b">Poros</span> · Open hardware markers excluded from accuracy summaries</p></figure>'
    )


def charts(benchmark, configs, prefix):
    figures = (panel(factors, points, benchmark, f"{prefix}-{index}")
               for index, (factors, points) in enumerate(configs))
    return '<div class="charts">' + "".join(figures) + '</div>'


def card_charts(benchmark, panels, card_id):
    configs = [(json.loads(key), points) for key, points in panels.items()]
    panel_factors = benchmark.get("panel_factors")
    if panel_factors is None:
        return charts(benchmark, configs, f"{card_id}-plot")
    if not isinstance(panel_factors, list) or not all(isinstance(name, str) for name in panel_factors):
        raise ValueError("panel_factors must be a list of factor names")

    choices = {}
    for factors, points in configs:
        selected = {name: value for name, value in factors.items() if name not in panel_factors}
        visible = {name: value for name, value in factors.items() if name in panel_factors}
        key = json.dumps(selected, sort_keys=True)
        choices.setdefault(key, []).append((visible, points))
    if len(choices) == 1:
        return charts(benchmark, configs, f"{card_id}-plot")

    menu = []
    views = []
    for index, (key, configs) in enumerate(choices.items()):
        label = factor_label(json.loads(key)) or "Default"
        menu.append(f'<option value="{index}">{esc(label)}</option>')
        hidden = ' hidden' if index else ''
        views.append(f'<div data-configuration="{index}"{hidden}>'
                     + charts(benchmark, configs, f"{card_id}-plot-{index}") + '</div>')
    return ('<label class="dashboard-configuration">Configuration '
            '<select data-dashboard-configuration>' + "".join(menu) + '</select></label>'
            '<div class="dashboard-panels">' + "".join(views) + '</div>')


def campaign_status(progress):
    completed, total = progress["completed"], progress["planned"]
    active, failed = progress["active"], progress["failed"]
    content = (
        f'<p><progress role="progressbar" aria-label="Evaluation progress" '
        f'value="{completed}" max="{total}"></progress> '
        f'{completed:,}/{total:,} complete · {len(active)} running · {len(failed)} failed</p>'
        '<nav><a href="pairs.csv">Paired results</a> · <a href="m5-trmm-sweep.csv">M5 Max TRMM sweep</a> · '
        '<a href="details.html">Full diagnostic details</a> · <a href="summary.json">Application summary</a></nav>'
    )
    for label, rows in (("Running inputs", active), ("Failed inputs", failed)):
        if rows:
            items = "".join(f"<li>{esc(item)}</li>" for item in rows)
            content += f"<details><summary>{label}</summary><ul>{items}</ul></details>"
    return content


def render(data):
    grouped = group(data)
    progress = data.get("progress")
    title = data.get("title", "Poros benchmark dashboard")
    labels = data.get("devices", {})
    content = STYLES + f"<header><h1>{esc(title)}</h1>"
    if data.get("description"):
        content += f'<p class="muted">{esc(data["description"])}</p>'
    if progress:
        content += campaign_status(progress)
    if not grouped:
        return page(title, content + "</header><p>No benchmark points supplied.</p>", footer="Poros")

    content += ('<div class="dashboard-global">' + y_switch("global", "Y scale for all charts")
                + '<p class="muted dashboard-wheel-hint">Scroll over a plot to zoom horizontally.</p></div></header>')
    device_order = [device for device in labels if device in grouped]
    device_order += [device for device in grouped if device not in labels]
    views = [(device, f"device-{index}") for index, device in enumerate(device_order, 1)]
    content += (
        view_style(["overview", *(view for _, view in views)])
        + '<div class="report"><nav class="views" aria-label="Report views">'
        '<a href="#overview">Overview</a>'
    )
    for device, view in views:
        content += f'<a href="#{view}">{esc(labels.get(device, device))}</a>'
    content += '</nav><section class="view" id="overview"><h2>Devices</h2><div class="dashboard-devices">'
    for device, view in views:
        count = len(grouped[device])
        label = "benchmark" if count == 1 else "benchmarks"
        content += (f'<a href="#{view}"><strong>{esc(labels.get(device, device))}</strong>'
                    f'<small>{count} {label}</small></a>')
    paired = {device: list(summary_points(grouped[device].values(), labels.get(device, device)))
              for device, _ in views}
    content += "</div>" + summary([point for points in paired.values() for point in points],
                                  "Accuracy across devices") + "</section>"
    for device, view in views:
        cards = list(grouped[device].values())
        content += (f'<section class="view" id="{view}">'
                    + summary(paired[device], f"{labels.get(device, device)} accuracy")
                    + '<nav class="dashboard-links" aria-label="Benchmarks">')
        for i, (benchmark, _) in enumerate(cards):
            content += f'<a href="#{view}-benchmark-{i}">{esc(benchmark["name"])}</a>'
        content += "</nav>"
        for i, (benchmark, panels) in enumerate(cards):
            card_id = f"{view}-benchmark-{i}"
            content += (f'<article class="dashboard-card" id="{card_id}">'
                        f'<h3 id="{card_id}-title">{esc(benchmark["name"])} <span class="badge">{KINDS[benchmark["kind"]]}</span></h3>'
                        + card_charts(benchmark, panels, card_id) + "</article>")
        content += "</section>"
    script = "<script>" + Path(__file__).with_suffix(".js").read_text(encoding="utf-8") + "</script>"
    content += "</div>"
    if progress:
        return live_document(title, content, progress["identity"], not progress["terminal"], VIEW_SCRIPT + script)
    return page(title, content, POINT_TIPS + VIEW_SCRIPT + script, footer="Poros · Benchmark timing")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("data", type=Path, help="Benchmark JSON")
    parser.add_argument("output", type=Path, help="Destination HTML")
    args = parser.parse_args()
    args.output.write_text(render(json.loads(args.data.read_text())), encoding="utf-8")


if __name__ == "__main__":
    main()
