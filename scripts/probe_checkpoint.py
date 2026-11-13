"""Atomic, identity-bound raw execution checkpoints for diagnostic reporting."""
import hashlib
import json
from pathlib import Path

import accuracy_progress as progress
import usertime as user


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()).hexdigest()


class Checkpoint:
    def __init__(self, directory, name, role, source, binary, revision, profile, elf_sha256, device=None):
        self.directory = Path(directory)
        self.name, self.role = name, role
        self.identity = dict(source=source, binary_sha256=user.digest(binary), revision=revision,
                             profile=profile, elf_sha256=elf_sha256, suite=name, role=role)
        # Devices that share parameters share a binary and a profile, so the
        # requested device is part of the key. Simulators that predate targets
        # keep their exact earlier keys.
        if device is not None:
            self.identity["device"] = device

    def filename(self, index, iterations):
        return f"{self.name}-{self.role}-{index}-{iterations}.json"

    def read(self, case, index, iterations, validate, *, path=None):
        """Strict replay: absence is an error, never permission to execute."""
        key = dict(self.identity, case=case, index=index, iterations=iterations, mode="ooo")
        path = Path(path) if path is not None else self.directory / self.filename(index, iterations)
        record = progress.site.read(path)
        if (record.get("schema") != 1 or record.get("key") != key
                or record.get("sha256") != digest({"key": key, "result": record.get("result")})):
            raise ValueError("probe checkpoint identity/hash mismatch: " + path.name)
        result = record["result"]
        validate(result)
        return result

    def __call__(self, case, index, iterations, collect, validate):
        path = self.directory / self.filename(index, iterations)
        if path.exists():
            return self.read(case, index, iterations, validate)
        result = collect()
        validate(result)
        key = dict(self.identity, case=case, index=index, iterations=iterations, mode="ooo")
        record = dict(schema=1, key=key, result=result, sha256=digest({"key": key, "result": result}))
        progress.atomic_write(path, record)
        return result
