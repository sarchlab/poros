# Grace native ELF layout

These 44 ELF files reproduce the binaries measured by the external GH200
acquisition in `evaluation/usertime/grace-native/summary.json`. They are linked
for 64 KiB pages. Their paths deliberately differ from the original fixtures:
M2/M1 references keep their existing ELF identities and address layout.

Run `python3 scripts/linux_reference.py --build` from the repository root to
rebuild from the exact recorded source revision and verify every measured
SHA-256. The toolchain used by the acquisition is documented in the Grace
measurement README. A compiler producing different bytes is rejected.
`python3 scripts/linux_reference.py --write` verifies the retained fixtures and
refreshes the plotting view without invoking a compiler.

The compact reference carries an explicit binary binding for each original
program. Both head and baseline simulator executions receive these same Grace
images. This changes workload layout, not the Grace target's M2 model parameters.
