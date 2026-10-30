# User-time accuracy report
Formula: `absolute-difference-over-min-v2`
Artifact: **pass**; hardware accuracy: **unverified**

| Case | Reference (s) | Quality | Base (s) | Head (s) | Base error % | Head error % | Change (pp) | Direction | Status / reason |
|---|---:|---|---:|---:|---:|---:|---:|---|---|
| integer-500000 | 0.000863 | invalid_nonpositive | 0.0004299641833810888 | 0.000429964183 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-1000000 | 0.00161 | noisy | 0.0008597650429799427 | 0.000859765043 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-2000000 | 0.003034 | noisy | 0.0017193638968481374 | 0.0017193639 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-4000000 | 0.004859 | noisy | 0.003438561604584527 | 0.0034385616 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-6000000 | 0.006245 | noisy | 0.005157759312320917 | 0.00515775931 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| integer-8000000 | 0.007701 | noisy | 0.006876957020057307 | 0.00687695702 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-8192 | 0.003214 | noisy | 0.004006840974212034 | 0.00400684097 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-65536 | 0.003595 | noisy | 0.004010950143266476 | 0.00401095014 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-262144 | 0.002446 | noisy | 0.004375605157593123 | 0.00437560516 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-1048576 | 0.003091 | noisy | 0.0044319137535816615 | 0.00443191375 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-8388608 | 0.003528 | noisy | 0.004957695988538682 | 0.00495769599 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| memory-33554432 | 0.003815 | noisy | 0.01618725329512894 | 0.0161872533 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-500000 | 0.002941 | noisy | 0.0020388696275071634 | 0.00203886963 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-1000000 | 0.004523 | noisy | 0.00407700458452722 | 0.00407700458 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-2000000 | 0.006676 | noisy | 0.008151969340974213 | 0.00815196934 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-4000000 | 0.01284 | noisy | 0.016302897134670487 | 0.0163028971 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-6000000 | 0.019145 | usable | 0.024453271346704872 | 0.0244532713 | 27.726671959806072 | 27.726671959806072 | 0.0 | slower | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| branch-8000000 | 0.024767 | usable | 0.03260393810888252 | 0.0326039381 | 31.64266204579692 | 31.64266204579692 | 0.0 | slower | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-8 | 0.000339 | invalid_nonpositive | 4.6074498567335245e-07 | 4.60744986e-07 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-63 | 0.000307 | invalid_nonpositive | 1.1631805157593124e-05 | 1.16318052e-05 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-120 | 0.000463 | invalid_nonpositive | 4.010888252148997e-05 | 4.01088825e-05 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-129 | 0.000452 | invalid_nonpositive | 4.617936962750716e-05 | 4.61793696e-05 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-400 | 0.001265 | invalid_nonpositive | 0.0004301022922636103 | 0.000430102292 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
| mvt-1536 | 0.008058 | noisy | 0.012336577077363896 | 0.0123365771 | unavailable | unverified | unavailable | unverified | unverified: reference environment/core/frequency not hardware-validated; no accuracy threshold approved |
calibration: `{"comparable": 0, "required": 12}`
held_out: `{"base": {"maximum_error_percent": 31.64266204579692, "mean_error_percent": 29.684667002801497}, "comparable": 2, "head": {"maximum_error_percent": 31.64266204579692, "mean_error_percent": 29.684667002801497}, "required": 12}`
