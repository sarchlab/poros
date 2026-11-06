"""Identify a main commit produced by a merged PR, including squash and rebase."""
import json
import os
import subprocess
import sys


def is_main_merge(repository, commit, pulls):
    return any(
        pr["merged_at"] and pr["merge_commit_sha"] == commit
        and pr["base"]["ref"] == "main" and pr["base"]["repo"]["full_name"] == repository
        for pr in pulls
    )


def main():
    repository, commit = sys.argv[1:]
    pages = json.loads(subprocess.check_output([
        "gh", "api", "--paginate", "--slurp",
        f"repos/{repository}/commits/{commit}/pulls?per_page=100",
    ]))
    merged = is_main_merge(repository, commit, (pr for page in pages for pr in page))
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        print(f"merged={str(merged).lower()}", file=output)


if __name__ == "__main__":
    main()
