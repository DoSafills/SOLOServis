"""Shared pytest setup.

Los artefactos de TAL-136 escriben en disco. Sin este aislamiento, correr la
suite crearía ``out/`` en el repositorio del desarrollador; cada test recibe
sus propias carpetas temporales.
"""

import pytest

import config as cfg


@pytest.fixture(autouse=True)
def isolated_artifacts(tmp_path, monkeypatch):
    monkeypatch.setattr(cfg, "ITEMS_DIR", str(tmp_path / "items"))
    monkeypatch.setattr(cfg, "RAW_DIR", str(tmp_path / "raw"))
    monkeypatch.setattr(cfg, "REJECTS_DIR", str(tmp_path / "rejected"))
    return {
        "items_dir": str(tmp_path / "items"),
        "raw_dir": str(tmp_path / "raw"),
        "rejects_dir": str(tmp_path / "rejected"),
    }
