"""Measure one head timing execution with the full workload's normal limits."""
import argparse
from pathlib import Path

import accuracy_parallel as parallel
import usertime as user


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("case_id")
    parser.add_argument("binary", type=Path)
    args = parser.parse_args()
    cases, outputs = parallel.inputs()
    case = next((case for case in cases if case["id"] == args.case_id), None)
    if case is None:
        parser.error("unknown accuracy case")
    state = user.source_state()
    if state["dirty"]:
        raise ValueError("expected clean measured source")
    profile = parallel.model_profiles.revision(parallel.ROOT, state["commit"])["profile"]
    result = user.execute(args.binary, case, expected_identity=outputs[case["id"]], expected_profile=profile)
    user.validate_timing(result, case)
    if result["build"].get("vcs.revision") != state["commit"] or result["build"].get("vcs.modified") != "false":
        raise ValueError("head binary provenance mismatch")
    print(f"{args.case_id}: instructions={result['instructions']} cycles={result['timing']['user_cycles']}")


if __name__ == "__main__":
    main()
