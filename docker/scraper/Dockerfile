# syntax=docker/dockerfile:1
#
# Build from the repository root so the spec directory and package metadata
# are both available:
#
#   docker build -f docker/scraper/Dockerfile -t soloservis/scraper:dev .
#
FROM python:3.12-slim AS build

ENV PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1

WORKDIR /build
COPY scraper/pyproject.toml scraper/README.md ./
COPY scraper/src ./src
RUN python -m pip install --upgrade pip \
    && python -m pip wheel --wheel-dir /wheels .

FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    SCRAPER_DATA_DIR=/data \
    SCRAPER_STORES_DIR=/app/stores

# curl_cffi (used by Scrapling's HTTP fetcher) needs libcurl at runtime.
RUN apt-get update \
    && apt-get install -y --no-install-recommends libcurl4 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /wheels /wheels
RUN python -m pip install --no-index --find-links=/wheels soloservis-scraper \
    && rm -rf /wheels

COPY scraper/stores /app/stores

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin scraper \
    && mkdir -p /data \
    && chown -R 10001:10001 /data

USER 10001:10001
WORKDIR /app
VOLUME ["/data"]
EXPOSE 8095

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8095/health', timeout=3).status == 200 else 1)"

CMD ["python", "-m", "scraper", "serve"]
