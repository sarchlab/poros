# M2 high-resolution pilot evidence

`m2-mvt-highres-2026-10-29.json` was collected on clean source commit
`7316013f53b0709dc77c1664927db8d0e3458aab` after the binary64 fused-instruction repair, using the default pilot command.
It uses mach_absolute_time, records raw ticks and timebase125/3ns, and observed
a minimum positive timer delta of1tick (~41.67ns). The observed minimum
timer-pair cost of0ticks means calls can land in the same clock tick; no overhead
is subtracted. n=8 is automatically flagged too short relative to clock resolution.

All six sizes passed exact object/linked-function byte identity and complete
native/simulated output checks against the independent mathematical reference.
The hardware
profile remains unvalidated, core/frequency unconfirmed, and cache states not
proven equivalent. Host load before/after was `{ 3.14 4.50 4.49 }`; exclusive idle
execution was not established. Error uses100*abs(sim-native)/native. The data
shows poor timing agreement but does not identify its cause.

| n | Native median batch mean (ns) | Simulated time (ns) | Absolute percentage error |
| --- | ---: | ---: | ---: |
| 8 | 52.917 | 212.034 | 300.695% |
| 63 | 4852.083 | 10994.556 | 126.595% |
| 120 | 22168.333 | 48314.040 | 117.942% |
| 129 | 26952.083 | 56658.453 | 110.219% |
| 400 | 363025.000 | 611252.722 | 68.378% |
| 1536 | 6136716.667 | 13032034.384 | 112.362% |

These numbers demonstrate the measurement/comparison system; they are not an accuracy acceptance result.

An earlier development clock source was microsecond-quantized and was discarded.
The retained evidence records clock resolution and flags short/noisy measurements.

Later changes canonicalize legacy registry input generation and strengthen tests
and report gates. They do not change this pilot's input generator, kernel, driver,
manifest, simulator arithmetic or timing model. The retained measurement remains
explicitly attributed to source commit 7316013, not to a later analysis commit.
