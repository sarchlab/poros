"""Atomic, identity-bound raw execution checkpoints for diagnostic reporting."""
import hashlib
import json
from pathlib import Path

import accuracy_progress as progress
import usertime as user


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()).hexdigest()


class Checkpoint:
    def __init__(self, directory, name, role, source, binary, revision, profile, elf_sha256):
        self.directory = Path(directory)
        self.name, self.role = name, role
        self.identity = dict(source=source, binary_sha256=user.digest(binary), revision=revision,
                             profile=profile, elf_sha256=elf_sha256, suite=name, role=role)

    def __call__(self, case, index, iterations, collect, validate):
        key = dict(self.identity, case=case, index=index, iterations=iterations, mode="ooo")
        path = self.directory / f"{self.name}-{self.role}-{index}-{iterations}.json"
        if path.exists():
            record = progress.site.read(path)
            if (record.get("schema") != 1 or record.get("key") != key
                    or record.get("sha256") != digest({"key": key, "result": record.get("result")})):
                raise ValueError("probe checkpoint identity/hash mismatch: " + path.name)
            result = record["result"]
            validate(result)
            return result
        result = collect()
        validate(result)
        record = dict(schema=1, key=key, result=result, sha256=digest({"key": key, "result": result}))
        progress.atomic_write(path, record)
        return result
