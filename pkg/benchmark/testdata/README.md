# Initial workload identities

`initial-state-go127-arm64.sha256` hashes register JSON followed by declared data
regions in registry order, before execution. It defines inputs, not correct outputs.
The 40 unchanged rows were captured at commit 9269bca on Go 1.27/darwin/arm64.
The Facesim row was deliberately replaced during M0 after its degenerate mesh was
corrected: 32 fixed nondegenerate tetrahedra cover all 20 vertices. The independent
CPU reference validates its rest inverses, deformation domain and finite forces.
No other initial fingerprint changed.

## MVT ELF migration

MVT now runs as a complete ELF program in the 24-case user-time suite. Its two
legacy CPU rows and one initial-state fingerprint were removed; all other rows
are unchanged. The remaining registry has 40 kernels (29 PolyBench and 11 PARSEC),
80 one/two-core baseline cases plus eight pipeline variants. Git history retains
the superseded MVT fixtures for rollback.
