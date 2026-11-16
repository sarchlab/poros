# Grace Deriche outlier investigation

Status: **investigation open; no fix and no accuracy change is claimed.**

Starting data is the completed three-device comparison of parent revision
`82e998387a52ad243b2390c7fc788d600a9437e2` (error = |predicted − reference| /
min). Two of the 237 qualified Grace application points exceed 50%:

| Point | GH200 reference (ms) | Model (ms) | Model / reference | Error |
|---|---:|---:|---:|---:|
| deriche-6 | 30.342 | 85.537494 | 2.8191 | 181.9112% |
| syrk-4 | 19.857 | 30.453765 | 1.5337 | 53.3654% |

The other five Deriche inputs are within 4.2 to 6.7% (6.10, 4.18, 6.68, 5.01
and 5.04% for inputs 1 to 5), so the outlier is one input of one kernel.
Before the Grace profile the same point was 28.7035% and syrk-4 3.0954%.

Scope of this work: establish the mechanism behind deriche-6 (syrk-4 is the
second failure and a control), and repair two report regressions (whole-page
reload, missing microbenchmark charts). A model change is proposed only if a
general mechanism is supported by evidence. The Grace parameters and their
sources are in [the model profile](model-profile.md#nvidia-grace-profile); the
reference times and their limits are in the
[GH200 measurement record](usertime/grace-native/README.md). References,
workload binaries and published report data are unchanged.
