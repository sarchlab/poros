# Project Specification

## What do you want to build?

M2Sim is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the Akita framework.

## How do you consider the project is success?

We need to support simulating the whole polybench benchmark suite with 20% average time estimation error. The maximum error of each benchmark execution cannot be greater than 50%. Error is calculated by error = abs(sim-hw)/min(sim,hw). 

You run on an Apple M2 machine, so you can run locally to measure.

## Cleanup Requirements (from human issue #116)

- Merge or close all PRs so that the main branch has the final version of the code
- Write README.md with instructions for how to use the simulator, technical details, and final accuracy achieved
