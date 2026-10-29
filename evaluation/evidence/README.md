# Initial M2 pilot evidence — rejected for timing calibration

Coordinator inspection found that CLOCK_MONOTONIC samples in this run were
quantized to1000ns, including zero-duration short ROIs. Preserve this dataset
as historical functional/pipeline evidence only; its timing estimates must not
be used for fitting or precision claims. A high-resolution replacement is
collected separately rather than overwriting these observations.

`m2-mvt-2026-10-29.json` was produced with:

```sh
go run ./cmd/evaluate pilot -out /tmp/poros-m0-m2-pilot.json
```

The source commit is `391bff918ec71555aae614210e1b570662421067`; the working tree
was clean. This later evidence-only commit preserves the exact evaluator, kernel,
manifest and configuration content used by that run. The report contains raw
individual timings, batch means, complete native outputs, byte identity and
source/compiler/binary hashes.

All six sizes passed native and simulated full-output checks against the
independent reference. The native executable and simulated kernel used identical
function bytes. No model parameters or historical reference times were changed.

| n | Data bytes | Instructions | Cycles | Native median batch mean (ns) | Simulated time (ns) | Absolute percentage error |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 8 | 768 | 971 | 740 | 60 | 212.03 | 253.39% |
| 63 | 33,768 | 52,616 | 38,371 | 5,280 | 10,994.56 | 108.23% |
| 120 | 119,040 | 189,131 | 168,616 | 22,410 | 48,314.04 | 115.59% |
| 129 | 137,256 | 218,408 | 197,738 | 25,540 | 56,658.45 | 121.84% |
| 400 | 1,292,800 | 2,086,411 | 2,133,272 | 332,600 | 611,252.72 | 83.78% |
| 1536 | 18,923,520 | 30,695,435 | 45,481,800 | 5,895,400 | 13,032,034.38 | 121.05% |

These results demonstrate that the evaluation path works, **not** that the model
is accurate. Timing agreement is poor and needs investigation during calibration.
The reported host load was `{ 6.74 7.81 6.82 }`; collection was separated from this
work's heavy test runs, but the machine was not proven idle. Core type and actual
frequency were unknown; model frequency was assumed to be 3.49 GHz. Timer
quantization/overhead is especially significant at n=8. Native and model cache
initial states and addresses are not guaranteed equivalent. These limitations
prevent treating this run as a validated, core-specific ground truth dataset.

Before fitting parameters, repeat measurements with stronger environmental
control and independently identified core conditions, assess variability and
cache policy, and retain this run as historical evidence. Do not replace it with
scaled timings or silently exclude the poorly agreeing sizes.

## Replacement high-resolution run

`m2-mvt-highres-2026-10-29.json` was collected on clean source commit
`fcba1fcafd6187d1f16821168196933f657bf2dc` using the same default pilot command.
It uses mach_absolute_time, records raw ticks and timebase125/3ns, and observed
a minimum positive timer delta of1tick (~41.67ns). The observed minimum
timer-pair cost of0ticks means calls can land in the same clock tick; no overhead
is subtracted. n=8 is automatically flagged too short relative to clock resolution.

All six sizes passed exact object/linked-function byte identity and complete
native/simulated output checks against the independent mathematical reference.
Instruction and cycle counts are unchanged from the first run. The hardware
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

These numbers demonstrate the measurement/comparison system; they are not an accuracy acceptance result. Retain the superseded coarse-clock run for audit only.
