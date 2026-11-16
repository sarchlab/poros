# Poros three-device accuracy report

Open `index.html` to view the completed report: 1,656 points across Grace, M1 and M2.

The results were measured at clean source `5c8ed4a683a2e3ba2808e904b63dd0608e0b1e83`; they retain that original source and binary identity. A separate release binding records the released commit and its executable-source equivalence. These are retained certified measurements, not a new release/CI execution.

All 723 qualified applications have maximum diff/min error below 50%, with each device median below 20%. The queue-only change leaves application predictions unchanged and improves twelve Apple capacity probe points. Other microbenchmarks still have substantial mismatches; `micro-over50.csv` lists 99 qualified device/probe observations above 50%. Grace's 288 diagnostics have no matching native reference and carry no hardware-accuracy claim.

Application mean/median/max errors (%): Grace 10.60688/7.87381/46.70768; M1 9.18362/5.65183/45.79034; M2 15.81650/14.41333/49.79176. Microbenchmarks are excluded. Error is `100*abs(prediction-reference)/min(prediction,reference)`.

The complete audit covers 2,520 retained original execution records, with zero execution failures. The local raw captures are retained by the project; this compact package includes the certificate, numeric summaries, plotting references and source provenance, not multi-gigabyte replay archives.

Existing native references are unchanged. The 43/38 multiply waiting bounds are effective model inferences, not physical queue-capacity measurements. SIMD integer MUL shares the existing integer classification but has not been separately calibrated by these GPR probes. M5 Max remains an M2 placeholder.

Additional model limits: Grace multiply-latency overrides do not cover every multiply alias/form. The stride-stream replacement path undercounts the diagnostic StreamEvictions statistic; that counter does not drive timing. Both are retained for a separate follow-up rather than changing the certified runtime during release.
