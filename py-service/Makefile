.PHONY: install test lint typecheck format clean

VENV ?= .venv
PY   := $(VENV)/bin/python

install:
	python -m venv $(VENV)
	$(PY) -m pip install --upgrade pip
	$(PY) -m pip install -e ".[dev]"

test:
	$(PY) -m pytest -q

lint:
	$(PY) -m ruff check src tests
	$(PY) -m mypy src

typecheck:
	$(PY) -m mypy src

format:
	$(PY) -m ruff format src tests

clean:
	rm -rf $(VENV) .pytest_cache .ruff_cache .mypy_cache build dist *.egg-info
