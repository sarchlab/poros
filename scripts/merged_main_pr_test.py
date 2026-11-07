#!/usr/bin/env python3
"""Only an exact merged PR targeting this repository's main enables accuracy."""
import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import merged_main_pr as gate


class MainMergeTests(unittest.TestCase):
    def setUp(self):
        self.repository = "sarchlab/poros"
        self.commit = "a" * 40
        self.pr = dict(merged_at="2026-11-06T00:00:00Z", merge_commit_sha=self.commit,
                       base=dict(ref="main", repo=dict(full_name=self.repository)))

    def test_only_exact_target_merge_qualifies(self):
        self.assertTrue(gate.is_main_merge(self.repository, self.commit, [self.pr]))
        self.assertFalse(gate.is_main_merge(self.repository, self.commit, []))
        for mutation in (lambda p: p.update(merged_at=None),
                         lambda p: p.update(merge_commit_sha="b" * 40),
                         lambda p: p["base"].update(ref="feature"),
                         lambda p: p["base"]["repo"].update(full_name="other/poros")):
            pr = copy.deepcopy(self.pr)
            mutation(pr)
            with self.subTest(pr=pr):
                self.assertFalse(gate.is_main_merge(self.repository, self.commit, [pr]))

    def test_later_api_page_and_negative_result_reach_workflow_output(self):
        for pages, expected in (([[], [self.pr]], "true"), ([[]], "false")):
            with self.subTest(expected=expected), tempfile.TemporaryDirectory() as folder:
                output = Path(folder) / "output"
                with patch.object(gate.sys, "argv", ["gate", self.repository, self.commit]), \
                     patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                     patch.object(gate.subprocess, "check_output", return_value=json.dumps(pages).encode()) as api:
                    gate.main()
                self.assertEqual(output.read_text(), f"merged={expected}\n")
                self.assertEqual(api.call_args.args[0], ["gh", "api", "--paginate", "--slurp",
                    f"repos/{self.repository}/commits/{self.commit}/pulls?per_page=100"])


if __name__ == "__main__":
    unittest.main()
