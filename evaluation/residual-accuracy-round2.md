# Residual accuracy, round 2

Status: **bootstrap. Nothing has been simulated and no model code or
parameter has been changed in this work.** This note records the verified
starting point and the residuals to be investigated. Findings and any change
will be added below as they are established.

## Starting point (retained; nothing rerun)

This work starts at `5ba7137486c31c0b16d93e7195706544b8563fca`, the merge of
[pull request 85](https://github.com/sarchlab/poros/pull/85). The
three-device comparison was measured at
`b6de5ca5d9f4fa2ade4053ce52765fcbc350a008` (1,656 points, 2,520 execution
records, certificate SHA-256
`15358deb6284a4d2f5204fd1c7419badb16f99caeec3af0b20fb0de96b4f5819`). The two
revisions differ in three files, all documents
(`evaluation/grace-deriche-investigation.md`,
`evaluation/grace-residual-investigation.md`, `evaluation/model-profile.md`),
so the `b6de5ca` records are the baseline of this work under their own
revision and binary identity. References, inputs, quality classes and
exclusions are unchanged.

Error is |predicted − reference| / min(predicted, reference). Recomputed
here from the retained rows:

| Qualified applications | Mean | Median | Maximum | Within 50% | Model / reference below 0.8 |
|---|---:|---:|---:|---:|---:|
| Grace, 237 | 10.61% | 7.87% | 46.71% | 237 | 10 |
| M1, 240 | 9.18% | 5.65% | 45.79% | 240 | 13 |
| M2, 246 | 15.82% | 14.41% | 49.79% | 246 | 17 |

The acceptance limits (each device's median at most 20%, every qualified
application within 50%) are met at the baseline and must stay met.

## Largest residuals at the baseline (retained)

Model / reference, error. "Low" means the model is faster than the hardware.

| Device | Inputs | Model / reference | Error |
|---|---|---:|---:|
| Grace | syr2k-3, syr2k-1 | 0.682, 0.712 (low) | 46.71%, 40.48% |
| Grace | atax-1 to atax-6 | 0.735 to 0.751 (low) | 33.21% to 36.13% |
| Grace | syrk-4, syrk-6 | 1.320, 1.316 (high) | 32.02%, 31.59% |
| Grace | symm-6, symm-1, mvt-3584 | 0.764, 0.831, 0.802 (low) | 30.95%, 20.38%, 24.62% |
| M1 | fdtd2d-1 to fdtd2d-6 | 0.686 to 0.712 (low) | 40.44% to 45.79% |
| M1 | vips-1 to vips-6 | 0.704 to 0.724 (low) | 38.06% to 42.06% |
| M1 | ferret-1 to ferret-6 | 1.354 to 1.356 (high) | 35.44% to 35.62% |
| M2 | jacobi1d-1 to jacobi1d-6 | 1.458 to 1.498 (high) | 45.83% to 49.79% |
| M2 | streamcluster-1 to streamcluster-6 | 0.673 to 0.723 (low) | 38.23% to 48.49% |
| M2 | gemm-1 to gemm-6 | 1.422 to 1.472 (high) | 42.24% to 47.15% |
| M2 | nussinov-1, nussinov-3 to nussinov-6 | 1.325 to 1.400 (high) | 32.45% to 39.95% |

## Plan

1. Group these residuals by plausible bottleneck from the retained records,
   the committed diagnostics and the workload code, without simulating.
2. Choose the cause that is best supported and most general, and test it
   with a small number of bounded runs, each checked for output and
   instruction parity.
3. Keep a change only if it is a model mechanism or profile value with
   stated evidence, and report every input it makes worse.

No benchmark-specific rule, reference change or exclusion is in scope. Grace
has no accessible machine and no native diagnostic references; its
application times are a collaborator's measurements.
