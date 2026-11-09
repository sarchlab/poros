"""Ordered complete/subset row checks shared by report producers and renderers."""


def covered_cases(rows, declared, *, partial=False):
    if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
        raise ValueError("invalid report rows")
    ids = [row.get("id") for row in rows]
    expected = [case["id"] for case in declared]
    if ids != ([ident for ident in expected if ident in ids] if partial else expected):
        raise ValueError("missing/duplicate/unknown/reordered report rows")
    by_id = {case["id"]: case for case in declared}
    return [by_id[ident] for ident in ids]
