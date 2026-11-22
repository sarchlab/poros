(() => {
  "use strict";
  let charts = [];
  const bounds = { left: 72, right: 413, top: 24, bottom: 235 };
  const svgNS = "http://www.w3.org/2000/svg";
  const state = new WeakMap();
  let globalChoices = [];
  const configurations = new Map();
  const format = new Intl.NumberFormat("en", { maximumSignificantDigits: 5, useGrouping: false });

  function svgNode(tag, attrs, text) {
    const node = document.createElementNS(svgNS, tag);
    for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, value);
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function getState(figure) {
    if (state.has(figure)) return state.get(figure);
    const data = JSON.parse(figure.dataset.chart);
    const first = data.points[0][0], last = data.points[data.points.length - 1][0];
    const padding = first === last ? Math.max(0.5, Math.abs(first) * 0.01) : 0;
    const entry = { data, fullX: [first - padding, last + padding], x: null, yScale: "linear" };
    state.set(figure, entry);
    return entry;
  }

  function draw(figure) {
    const entry = getState(figure);
    const { data } = entry;
    const svg = figure.querySelector("svg");
    const log = entry.yScale === "log";
    const [x0, x1] = entry.x || entry.fullX;
    figure.querySelector("[data-reset-x]").disabled = !entry.x;
    svg.replaceChildren();

    const transform = log ? Math.log10 : value => value;
    const series = data.series.map((_, index) => data.points.map(row => {
      const value = row[index + 1];
      if (value === null || (log && value <= 0)) return null;
      return { x: row[0], y: transform(value), value, label: row[3] };
    }));
    const values = [];
    const inRange = x => x >= x0 && x <= x1;
    for (const points of series) {
      let previous = null;
      for (const point of points) {
        if (point && inRange(point.x)) values.push(point.y);
        if (point && previous) {
          for (const edge of [x0, x1]) {
            if (previous.x < edge && edge < point.x) {
              const ratio = (edge - previous.x) / (point.x - previous.x);
              values.push(previous.y + ratio * (point.y - previous.y));
            }
          }
        }
        previous = point;
      }
    }
    let y0 = log && values.length ? Math.min(...values) : 0;
    let y1 = values.length ? Math.max(...values) : 1;
    if (y0 === y1) {
      if (log) { y0 -= 0.5; y1 += 0.5; }
      else y1 = 1;
    }
    const { left, right, top, bottom } = bounds;
    const px = x => left + (x - x0) / (x1 - x0) * (right - left);
    const py = y => bottom - (y - y0) / (y1 - y0) * (bottom - top);
    const tickDecimals = Math.max(0, Math.min(12, Math.ceil(-Math.log10((x1 - x0) / 4))));
    const xFormat = entry.x
      ? new Intl.NumberFormat("en", { maximumFractionDigits: tickDecimals, useGrouping: false })
      : format;
    const label = (x, y, text, anchor = "middle") =>
      svg.append(svgNode("text", { x, y, "text-anchor": anchor }, text));

    svg.append(svgNode("path", { d: `M${left} ${top}V${bottom}H${right}`, fill: "none", stroke: "#bacbd0" }));
    for (let i = 0; i <= 4; i++) {
      const unit = i / 4;
      const y = bottom - unit * (bottom - top);
      const value = y0 + unit * (y1 - y0);
      svg.append(svgNode("path", { d: `M${left} ${y}H${right}`, stroke: "#dae4e6" }));
      if (values.length) label(left - 6, y + 4, format.format(log ? 10 ** value : value), "end");
      if (data.points.length > 1) {
        const x = x0 + unit * (x1 - x0);
        label(px(x), bottom + 18, xFormat.format(x));
      }
    }
    if (data.points.length === 1) {
      const x = data.points[0][0];
      label(px(x), bottom + 18, xFormat.format(x));
    }
    label((left + right) / 2, 278, data.xLabel);
    label(6, 12, `Time (${data.unit}${log ? ", log scale" : ""})`, "start");
    if (!values.length) label((left + right) / 2, 127, log ? "No positive timings in range" : "No timings in range");

    const clip = svgNode("clipPath", { id: svg.id + "-clip" });
    // Leave room for the stroke at the two Y limits.
    clip.append(svgNode("rect", { x: left, y: top - 1, width: right - left, height: bottom - top + 2 }));
    const lines = svgNode("g", { "clip-path": `url(#${clip.id})` });
    svg.append(clip, lines);

    const markers = new Map();
    data.series.forEach(([, name, color], index) => {
      let segment = [];
      function drawSegment() {
        if (segment.length > 1)
          lines.append(svgNode("polyline", { fill: "none", stroke: color, points: segment.join(" ") }));
        segment = [];
      }
      for (const point of series[index]) {
        if (!point) {
          drawSegment();
          continue;
        }
        const x = px(point.x), y = py(point.y);
        const key = `${x},${y}`;
        segment.push(key);
        if (!inRange(point.x)) continue;
        const mark = markers.get(key) || { x, y, color, labels: [] };
        mark.labels.push(`${name}: ${point.label} · ${format.format(point.value)} ${data.unit}`);
        markers.set(key, mark);
      }
      drawSegment();
    });
    for (const mark of markers.values()) {
      const tip = mark.labels.join("\n");
      svg.append(svgNode("circle", {
        cx: mark.x, cy: mark.y, r: 3, fill: mark.color, tabindex: 0,
        "data-tip": tip, "aria-label": tip, stroke: "transparent", "stroke-width": 4
      }));
    }
  }

  function drawVisible() {
    // On a direct fragment link, the browser can apply :target after this
    // script runs. Find the device from the URL without waiting for layout.
    let target;
    try {
      target = document.getElementById(decodeURIComponent(location.hash.slice(1)));
    } catch {
      // An invalid fragment uses the default device.
    }
    const view = target?.closest(".view") || document.getElementById("overview");
    for (const figure of view.querySelectorAll(".dashboard-chart")) {
      if (!figure.closest("[hidden]")) draw(figure);
    }
  }

  document.addEventListener("change", event => {
    const control = event.target;
    if (control.matches("[data-dashboard-configuration]")) {
      configurations.set(control.closest(".dashboard-card").id, control.value);
      applyConfiguration(control);
      drawVisible();
    }
  });

  function setScale(figure, scale) {
    getState(figure).yScale = scale;
    for (const choice of figure.querySelectorAll("[data-chart-y]"))
      choice.setAttribute("aria-pressed", String(choice.dataset.chartY === scale));
  }

  document.addEventListener("click", event => {
    const button = event.target.closest("button");
    if (!button) return;
    if (button.matches("[data-global-y]")) {
      for (const choice of globalChoices)
        choice.setAttribute("aria-pressed", String(choice === button));
      for (const figure of charts) setScale(figure, button.dataset.globalY);
      drawVisible();
    }
    if (button.matches("[data-chart-y]")) {
      const figure = button.closest("figure");
      setScale(figure, button.dataset.chartY);
      for (const choice of globalChoices) {
        const matches = Array.from(charts).every(chart =>
          (state.get(chart)?.yScale || "linear") === choice.dataset.globalY);
        choice.setAttribute("aria-pressed", String(matches));
      }
      draw(figure);
    }
    if (button.matches("[data-reset-x]")) {
      const figure = button.closest("figure");
      getState(figure).x = null;
      draw(figure);
    }
  });

  function applyConfiguration(control) {
    for (const panel of control.closest(".dashboard-card").querySelectorAll("[data-configuration]"))
      panel.hidden = panel.dataset.configuration !== control.value;
  }

  function initializeCharts() {
    const previous = new Map(charts.map(figure => [figure.id, state.get(figure)]));
    charts = Array.from(document.querySelectorAll(".dashboard-chart"));
    globalChoices = document.querySelectorAll("[data-global-y]");
    for (const control of document.querySelectorAll("[data-dashboard-configuration]")) {
      const selection = configurations.get(control.closest(".dashboard-card").id);
      if (selection !== undefined) {
        control.value = selection;
        applyConfiguration(control);
      }
    }
    charts.forEach(figure => {
      figure.querySelector("svg").id = `${figure.id}-svg`;
      const saved = previous.get(figure.id);
      state.delete(figure);
      if (saved) {
        setScale(figure, saved.yScale);
        getState(figure).x = saved.x;
      }
    });
    for (const choice of globalChoices) {
      const matches = charts.every(chart => (state.get(chart)?.yScale || "linear") === choice.dataset.globalY);
      choice.setAttribute("aria-pressed", String(matches));
    }
    drawVisible();
  }

  document.addEventListener("wheel", event => {
    const svg = event.target.closest(".dashboard-chart svg");
    if (!svg) return;
    const figure = svg.closest("figure");
    if (!event.deltaY || event.ctrlKey || Math.abs(event.deltaX) > Math.abs(event.deltaY)) return;
    const { left, right, top, bottom } = bounds;
    const { x, y } = new DOMPoint(event.clientX, event.clientY)
      .matrixTransform(svg.getScreenCTM().inverse());
    if (x < left || x > right || y < top || y > bottom) return;

    const entry = getState(figure);
    if (entry.data.points.length < 2) return;
    const [min, max] = entry.fullX;
    const [start, end] = entry.x || entry.fullX;
    const full = max - min, span = end - start;
    const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? 440 : 1);
    const factor = Math.exp(Math.min(1, Math.max(-1, delta * 0.004)));
    const next = Math.min(full, Math.max(full / 10000, span * factor));
    if (Math.abs(next - span) <= full * 1e-12) return;

    event.preventDefault();
    const anchor = (x - left) / (right - left);
    const nextStart = Math.max(min, Math.min(max - next, start + (span - next) * anchor));
    entry.x = next === full ? null : [nextStart, nextStart + next];
    draw(figure);
  }, { passive: false });

  addEventListener("hashchange", drawVisible);
  document.addEventListener("report-before-scroll", initializeCharts);
  initializeCharts();
})();
