# User-time accuracy report
Formula: `absolute-difference-over-min-v2`
Artifact: **pass**; hardware accuracy: **unverified**

| Case | Reference (s) | Quality | Base (s) | Head (s) | Base error % | Head error % | Change (pp) | Direction | Status / reason |
|---|---:|---|---:|---:|---:|---:|---:|---|---|
| integer-2000000 | 0.002834 | noisy | 0.0017193570200573066 | 0.00171935702 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-8000000 | 0.008054 | usable | 0.006876950143266476 | 0.00687695014 | 17.11587014900822 | 17.11587014900822 | 0.0 | faster | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-2000000 | 0.002235 | noisy | 0.008027783381088826 | 0.00802778338 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-8000000 | 0.004865 | noisy | 0.032096551289398284 | 0.0320965513 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-2000000 | 0.006737 | noisy | 0.007747432378223495 | 0.00774743238 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-8000000 | 0.024877 | usable | 0.030989022922636104 | 0.0309890229 | 24.56897102800219 | 24.56897102800219 | 0.0 | slower | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
calibration: `{"base": {"maximum_error_percent": 17.11587014900822, "mean_error_percent": 17.11587014900822}, "comparable": 1, "head": {"maximum_error_percent": 17.11587014900822, "mean_error_percent": 17.11587014900822}, "required": 2}`
held_out: `{"base": {"maximum_error_percent": 24.56897102800219, "mean_error_percent": 24.56897102800219}, "comparable": 1, "head": {"maximum_error_percent": 24.56897102800219, "mean_error_percent": 24.56897102800219}, "required": 4}`
