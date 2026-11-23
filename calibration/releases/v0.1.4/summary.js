function comparisonPairs(rows, benchmarks) {
  const pairs = [];
  for (const point of rows) {
    const series = Object.entries(benchmarks[point.benchmark].values);
    const hardware = series.filter(([, value]) => value.role === 'hardware_measurement');
    for (const [id, simulated] of series) {
      if (simulated.role !== 'simulator_output') continue;
      const references = hardware.filter(([, value]) => value.unit === simulated.unit);
      if (references.length !== 1) continue;
      const [reference, measured] = references[0];
      const actual = point.values[reference];
      const prediction = point.values[id];
      if (!Number.isFinite(actual) || actual <= 0 || !Number.isFinite(prediction) || prediction <= 0) continue;
      pairs.push({
        point,
        group: point.group,
        actual,
        prediction,
        signedError: 100 * (prediction - actual) / Math.min(prediction, actual),
        unit: simulated.unit,
        id: JSON.stringify([reference, id]),
        name: simulated.name + ' vs. ' + measured.name,
      });
    }
  }
  return pairs.filter(p => Number.isFinite(p.signedError));
}

function errorStats(pairs) {
  if (!pairs.length) return [];
  const errors = pairs.map(pair => Math.abs(pair.signedError));
  const mean = errors.reduce((total, error) => total + error, 0) / errors.length;
  const max = Math.max(...errors);
  return [['Average error', mean.toFixed(2) + '%'], ['Max error', max.toFixed(2) + '%']];
}

function summaryAxes(title, xLabel, yLabel, xRange, yRange, xOptions = {}, yOptions = {}) {
  const canvas = el('canvas', 'curve');
  canvas.width = 640;
  canvas.height = 372;
  canvas.setAttribute('role', 'img');
  canvas.setAttribute('aria-label', title);
  const ctx = canvas.getContext('2d');
  ctx.scale(2, 2);
  ctx.font = '10px system-ui';
  const frame = {left: 51, right: 306, top: 12, bottom: 143};
  const {left, right, top, bottom} = frame;
  const xAxis = numericAxis(xRange, [left, right], {count: 6, ...xOptions});
  const yAxis = numericAxis(yRange, [bottom, top], yOptions);
  const x = xAxis.position;
  const y = yAxis.position;
  drawAxisTicks(ctx, xAxis, yAxis, frame);
  drawAxisTitles(ctx, xLabel, yLabel, frame, 179);
  return {canvas, ctx, x, y};
}

function summaryPoint(chart, x, y, color) {
  chart.ctx.fillStyle = color;
  chart.ctx.beginPath();
  chart.ctx.arc(chart.x(x), chart.y(y), 2.15, 0, 2 * Math.PI);
  chart.ctx.fill();
}

function referenceLine(chart, x0, y0, x1, y1) {
  const {ctx, x, y} = chart;
  ctx.save();
  ctx.strokeStyle = '#69808a';
  ctx.setLineDash([4, 3]);
  ctx.beginPath();
  ctx.moveTo(x(x0), y(y0));
  ctx.lineTo(x(x1), y(y1));
  ctx.stroke();
  ctx.restore();
}

function rankedErrors(pairs, colors) {
  const ranked = [...pairs].sort((a, b) => a.signedError - b.signedError);
  const end = Math.max(2, ranked.length);
  const minimum = Math.min(0, ranked[0].signedError);
  const maximum = Math.max(0, ranked[ranked.length - 1].signedError);
  const margin = Math.max(0.2, (maximum - minimum) * 0.06);
  const chart = summaryAxes(
    'S-curve of signed relative error',
    'Case rank', 'Signed error (%)', [1, end], [minimum - margin, maximum + margin],
    {count: Math.min(6, end)}, {format: n => (n > 0 ? '+' : '') + formatTick(n) + '%'}
  );
  referenceLine(chart, 1, 0, end, 0);
  ranked.forEach((pair, i) => summaryPoint(chart, i + 1, pair.signedError, colors.get(pair.id).color));
  return chart.canvas;
}

function absoluteErrorCDF(pairs) {
  const errors = pairs.map(pair => Math.abs(pair.signedError)).sort((a, b) => a - b);
  const log = preferLogScale(errors);
  const left = log ? 10 ** Math.floor(Math.log10(errors[0])) : 0;
  const maximum = errors.at(-1);
  const right = log ? 10 ** (Math.ceil(Math.log10(maximum)) + 0.1) : maximum * 1.04 || 1;
  const chart = summaryAxes(
    'Absolute error cumulative distribution',
    'Absolute error (%)' + (log ? ' · log' : ''), 'Cumulative fraction', [left, right], [0, 1],
    {log}
  );
  const {ctx, x, y} = chart;
  ctx.strokeStyle = palette[0];
  ctx.lineWidth = 1.7;
  ctx.beginPath();
  ctx.moveTo(x(left), y(0));
  let fraction = 0;
  for (let i = 0; i < errors.length; i++) {
    const px = x(errors[i]);
    ctx.lineTo(px, y(fraction));
    fraction = (i + 1) / errors.length;
    ctx.lineTo(px, y(fraction));
  }
  ctx.lineTo(x(right), y(1));
  ctx.stroke();
  const nearestRank = p => errors[Math.ceil(p * errors.length) - 1].toPrecision(3) + '%';
  const percentiles = [50, 90, 95, 98].map(p => ['P' + p, nearestRank(p / 100)]);
  const details = plotDetails(
    [el('p', '', '|simulated − measured| / min(simulated, measured)')],
    percentiles,
    pairs.length
  );
  details.append(renderStats(errorStats(pairs), pairs.length));
  return [chart.canvas, details];
}

const scatterLogPreferences = new Map();

function averageRanks(values) {
  const ordered = values.map((value, index) => ({value, index})).sort((a, b) => a.value - b.value);
  const ranks = [];
  for (let start = 0; start < ordered.length;) {
    let end = start + 1;
    while (end < ordered.length && ordered[end].value === ordered[start].value) end++;
    const rank = (start + end + 1) / 2;
    for (let i = start; i < end; i++) ranks[ordered[i].index] = rank;
    start = end;
  }
  return ranks;
}

function rankStatistics(pairs) {
  let concordant = 0, discordant = 0, measuredUnequal = 0, simulatedUnequal = 0;
  for (let i = 0; i < pairs.length; i++) {
    for (let j = 0; j < i; j++) {
      const x = Math.sign(pairs[i].actual - pairs[j].actual);
      const y = Math.sign(pairs[i].prediction - pairs[j].prediction);
      if (x) measuredUnequal++;
      if (y) simulatedUnequal++;
      if (x && y) {
        if (x === y) concordant++;
        else discordant++;
      }
    }
  }
  const tauDenominator = Math.sqrt(measuredUnequal * simulatedUnequal);
  const measuredRanks = averageRanks(pairs.map(pair => pair.actual));
  const simulatedRanks = averageRanks(pairs.map(pair => pair.prediction));
  const mean = (pairs.length + 1) / 2;
  let covariance = 0, measuredVariance = 0, simulatedVariance = 0;
  for (let i = 0; i < pairs.length; i++) {
    const x = measuredRanks[i] - mean;
    const y = simulatedRanks[i] - mean;
    covariance += x * y;
    measuredVariance += x * x;
    simulatedVariance += y * y;
  }
  const rhoDenominator = Math.sqrt(measuredVariance * simulatedVariance);
  return {
    tau: tauDenominator ? (concordant - discordant) / tauDenominator : null,
    concordant,
    measuredUnequal,
    rho: rhoDenominator ? covariance / rhoDenominator : null,
  };
}

function rankingStats(pairs) {
  const stats = rankStatistics(pairs);
  const probability = stats.measuredUnequal
    ? (100 * stats.concordant / stats.measuredUnequal).toFixed(2) + '% (' +
      stats.concordant.toLocaleString() + '/' + stats.measuredUnequal.toLocaleString() + ')'
    : '—';
  return [
    ['Kendall’s τ-b', stats.tau?.toFixed(4) ?? '—'],
    ['Pairwise ordering accuracy', probability],
    ['Spearman’s ρ', stats.rho?.toFixed(4) ?? '—'],
  ];
}

function comparisonScatter(pairs, unit, colors, log) {
  const values = pairs.flatMap(pair => [pair.actual, pair.prediction]);
  const transform = log ? Math.log10 : n => n;
  const inverse = log ? n => 10 ** n : n => n;
  const low = log ? transform(Math.min(...values)) : 0;
  const high = transform(Math.max(...values));
  const margin = log ? Math.max((high - low) * 0.04, 0.08) : high * 0.04;
  const range = [log ? inverse(low - margin) : 0, inverse(high + margin)];
  const axisUnit = [displayUnit(unit), log ? 'log' : ''].filter(Boolean).join(', ');
  const suffix = axisUnit ? ' (' + axisUnit + ')' : '';
  const chart = summaryAxes(
    'Measured versus predicted values, ' + (log ? 'logarithmic' : 'linear') + ' X and Y axes',
    'Measured' + suffix, 'Simulated' + suffix, range, range, {log, unit}, {log, unit}
  );
  referenceLine(chart, range[0], range[0], range[1], range[1]);
  for (const pair of pairs) {
    summaryPoint(chart, pair.actual, pair.prediction, colors.get(pair.id).color);
  }
  return chart.canvas;
}

function scatterWithScale(key, pairs, unit, colors) {
  const plot = el('div', 'plot');
  const controls = el('div', 'scale-controls');
  const button = el('button', '', 'X/Y log');
  button.type = 'button';
  let log = scatterLogPreferences.get(key) ?? preferLogScale(pairs.flatMap(pair => [pair.actual, pair.prediction]));
  button.setAttribute('aria-pressed', String(log));
  let canvas = comparisonScatter(pairs, unit, colors, log);
  button.addEventListener('click', () => {
    log = !log;
    scatterLogPreferences.set(key, log);
    button.setAttribute('aria-pressed', String(log));
    const next = comparisonScatter(pairs, unit, colors, log);
    canvas.replaceWith(next);
    canvas = next;
  });
  controls.append(button);
  plot.append(controls, canvas, plotDetails([
    el('p', '', 'Dashed line indicates equal values.'),
    el('p', '', 'Pairwise accuracy uses pairs with distinct measured values.'),
  ], [...rankingStats(pairs), ...errorStats(pairs)], pairs.length));
  return plot;
}

function renderSummary(pairs, scope, groups) {
  const section = document.querySelector('#summary');
  section.replaceChildren();
  section.hidden = !pairs.length;
  if (!pairs.length) return;
  if (scope === null && new Set(pairs.map(pair => pair.group)).size > 1) {
    pairs = pairs.map(pair => ({
      ...pair,
      id: JSON.stringify([pair.group, pair.id]),
      name: groups[pair.group].name + ' · ' + pair.name,
    }));
  }

  const colors = new Map();
  for (const pair of pairs) {
    if (!colors.has(pair.id)) {
      colors.set(pair.id, {color: palette[colors.size % palette.length], name: pair.name});
    }
  }

  const legend = el('div', 'legend summary-legend');
  for (const {color, name} of colors.values()) {
    const item = el('span');
    const swatch = el('i', 'key');
    swatch.style.backgroundColor = color;
    item.append(swatch, document.createTextNode(name));
    legend.append(item);
  }
  const charts = el('div', 'charts summary-charts');
  const signed = el('figure', 'card');
  signed.append(el('figcaption', '', 'S-curve'), rankedErrors(pairs, colors),
    plotDetails([el('p', '', 'Negative values are underestimates. Positive values are overestimates.')],
      errorStats(pairs), pairs.length));
  const cdf = el('figure', 'card');
  cdf.append(el('figcaption', '', 'Absolute error distribution (CDF)'), ...absoluteErrorCDF(pairs));
  const scatter = el('figure', 'card');
  scatter.append(el('figcaption', '', 'Measured vs. simulated'));
  for (const [unit, values] of Map.groupBy(pairs, pair => pair.unit)) {
    const key = JSON.stringify([scope, unit]);
    scatter.append(scatterWithScale(key, values, unit, colors));
  }
  charts.append(signed, cdf, scatter);
  const heading = scope === null ? 'Overview · Accuracy summary' : groups[scope].name + ' accuracy summary';
  const context = el('div', 'summary-context');
  context.append(el('p', 'summary-note', pairs.length.toLocaleString() + ' paired measurements'), legend);
  section.append(el('h2', '', heading), context, charts);
}
