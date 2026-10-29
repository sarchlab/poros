# M2 high-resolution pilot evidence

`m2-mvt-highres-2026-10-29.json` was collected on clean source commit
`fcba1fcafd6187d1f16821168196933f657bf2dc` using the same default pilot command.
It uses mach_absolute_time, records raw ticks and timebase125/3ns, and observed
a minimum positive timer delta of1tick (~41.67ns). The observed minimum
timer-pair cost of0ticks means calls can land in the same clock tick; no overhead
is subtracted. n=8 is automatically flagged too short relative to clock resolution.

All six sizes passed exact object/linked-function byte identity and complete
native/simulated output checks against the independent mathematical reference.
The hardware
profile remains unvalidated, core/frequency unconfirmed, and cache states not
proven equivalent. Host load before/after was `{ 3.83 4.52 4.67 }`; exclusive idle
execution was not established. Error uses100*abs(sim-native)/native. The data
shows poor timing agreement but does not identify its cause.

| n | Native median batch mean (ns) | Simulated time (ns) | Absolute percentage error |
| --- | ---: | ---: | ---: |
| 8 | 63.333 | 212.034 | 234.791% |
| 63 | 5780.417 | 10994.556 | 90.204% |
| 120 | 23167.083 | 48314.040 | 108.546% |
| 129 | 25575.417 | 56658.453 | 121.535% |
| 400 | 330141.667 | 611252.722 | 85.149% |
| 1536 | 5609458.333 | 13032034.384 | 132.323% |

These numbers demonstrate the measurement/comparison system; they are not an accuracy acceptance result.

An earlier development clock source was microsecond-quantized and was discarded.
The retained evidence records clock resolution and flags short/noisy measurements.
