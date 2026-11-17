# Apple M1 and M2 Linux performance-core references

The M1 (`marin-0`) and M2 (`marin-5`) captures each executed 264 whole-program inputs. Both used the same 44 ARM64 Linux ELF fixtures, built from `7031aa3b432cc8a1c83b11f6c0d732cb0a943022` with 16 KiB load-section alignment. The captures use `wait4.ru_utime` with one warmup and 21 recorded samples per input. Every sample exited successfully and its complete output length and SHA-256 matched the original workload contract.

Both Linux collectors ran via `taskset -c 4`. The capture metadata records the only allowed CPU, its frequency policy and the topology used to identify the fastest cluster. The collector checked the pinned CPU and performance governor throughout capture. `cpuinfo_max_freq` identifies the cluster. Instantaneous frequency was unmeasured.

The application reference for each Apple device is derived from its compressed raw log, capture metadata and the exact shared ELF bytes in `tests/apple-pcore/`. The existing 20% P90–P10 spread limit still applies. The 18 legacy whole-program microbenchmarks are kept outside application accuracy statistics. The six specialized diagnostic suites have separate measurement sources.

From the repository root, run `python3 scripts/pcore_reference.py m1` and `python3 scripts/pcore_reference.py m2` to rederive and verify the compact references. Add `--write` to update the derived plotting data from the retained evidence.
