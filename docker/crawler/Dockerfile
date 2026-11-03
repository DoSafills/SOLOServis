# syntax=docker/dockerfile:1
# Crawler + artefactos por ejecución (TAL-136).
# Build context: crawler/  (docker build -f docker/crawler/Dockerfile crawler)
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

# UID/GID numéricos: Kubernetes puede verificar runAsNonRoot y el PVC se monta
# con fsGroup 10001.
RUN groupadd -g 10001 appgroup \
    && useradd -u 10001 -g appgroup -m -s /usr/sbin/nologin appuser

WORKDIR /app

# Solo el proyecto (sin tests ni venv) para que la capa de dependencias sea
# cacheable y la imagen no arrastre basura local.
COPY pyproject.toml README.md ./
COPY src ./src
COPY scripts ./scripts
COPY examples ./examples
COPY schema ./schema

RUN pip install --upgrade pip \
    && pip install .

# SQLite, JSONL y HTML crudo viven en el volumen persistente (TAL-144).
ENV SCRAPER_DB_PATH=/data/scraper.db \
    SCRAPER_ITEMS_DIR=/data/items \
    SCRAPER_RAW_DIR=/data/raw \
    SCRAPER_REJECTS_DIR=/data/rejected

RUN mkdir -p /data/items /data/raw /data/rejected \
    && chown -R 10001:10001 /data /app

USER 10001:10001

VOLUME ["/data"]

CMD ["python", "src/main.py"]
