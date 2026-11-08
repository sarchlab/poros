# M2 accuracy umbrella

The first increment establishes reproducible microbenchmark evidence. No hardware
width or latency parameter is changed by the suite itself.

1. Land the shared native/ELF throughput probes, functional CI and separate report
   section. Acceptance: all declared cases execute correctly, source identities
   validate, and microbenchmark results do not enter application accuracy metrics.
2. Use the NOP, independent-chain and mixed-resource results to distinguish
   frontend admission, execution-port throughput and dependency latency. Correct
   modeling defects that invalidate a probe before using it to select parameters.
   Eight-wide frontend throughput is a hypothesis; neither these steady-state
   tests nor an improved application fit alone establishes backend issue width.
3. Review each subsequent model change independently with targeted regression
   tests and microbenchmark evidence. Preserve the existing hardware reference
   when inputs/protocol are unchanged. After merging the umbrella into main, the
   existing full accuracy campaign reports application error, Pearson r,
   Kendall's tau-b and pairwise agreement without mixing in these probes.

Revert an individual model change if functional behavior changes or its claimed
microarchitectural effect cannot be reproduced. Application fit alone is not
sufficient justification to retain a physically unsupported parameter.
