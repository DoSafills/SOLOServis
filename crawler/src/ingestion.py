"""Cliente de Ingestion API (TAL-77) para el crawler.

El crawler produce ítems normalizados (:mod:`normalized`); este módulo los
entrega. Habla el contrato de ``backend/README.md`` §12.7:

* ``POST /ingestion/runs`` abre la ejecución (una por ``SCRAPE_RUN`` local);
* ``POST /ingestion/runs/{id}/items`` envía lotes de 1 a 100;
* ``POST /ingestion/runs/{id}/finish`` la cierra como ``success`` o ``failed``.

Tres decisiones sostienen el diseño:

* **Sin dependencias nuevas.** Es una API interna de JSON sobre HTTP; se usa
  ``urllib`` de la biblioteca estándar en ``asyncio.to_thread``. Scrapling no
  sirve aquí: su fetcher está pensado para HTML, no para POST autenticados.
* **Los errores del backend se leen, no se adivinan.** Ingestion responde
  ``application/problem+json`` (RFC 9457) con ``code``, ``detail`` y
  ``errors[]``; el cliente los conserva para que el crawler registre el motivo
  real en lugar de "HTTP 422".
* **Reintentar es seguro, pero no gratis.** El upsert de oferta es idempotente
  por producto + tienda + URL; aun así un lote reenviado puede inflar
  contadores. Se reintenta solo lo transitorio (408/429/5xx y red).
"""

import asyncio
import http.client
import json
from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen

import config as cfg
from utils import get_logger

_log = get_logger("INGEST")

TRANSIENT_STATUS = frozenset({408, 429, 500, 502, 503, 504})
MIN_TOKEN_LENGTH = 32  # authz ignora tokens más cortos


class IngestionError(Exception):
    """Un fallo de Ingestion API con el vocabulario del backend."""

    def __init__(
        self,
        message: str,
        *,
        status: int | None = None,
        code: str | None = None,
        field_errors: tuple[dict[str, Any], ...] = (),
        retriable: bool = False,
    ):
        super().__init__(message)
        self.status = status
        self.code = code
        self.field_errors = field_errors
        self.retriable = retriable

    def __str__(self) -> str:
        prefix = f"HTTP {self.status} " if self.status else ""
        code = f"[{self.code}] " if self.code else ""
        return f"{prefix}{code}{super().__str__()}"


@dataclass(slots=True)
class RunInfo:
    """Una ejecución de Ingestion (``scrape_run`` en PostgreSQL)."""

    id: int
    source: str
    status: str
    source_url: str | None = None
    store_id: int | None = None
    scraper_type: str | None = None
    records_found: int = 0
    records_processed: int = 0
    records_failed: int = 0

    @classmethod
    def from_payload(cls, payload: Mapping[str, Any]) -> "RunInfo":
        return cls(
            id=int(payload.get("id") or 0),
            source=str(payload.get("source") or ""),
            status=str(payload.get("status") or ""),
            source_url=payload.get("sourceUrl"),
            store_id=payload.get("storeId"),
            scraper_type=payload.get("scraperType"),
            records_found=int(payload.get("recordsFound") or 0),
            records_processed=int(payload.get("recordsProcessed") or 0),
            records_failed=int(payload.get("recordsFailed") or 0),
        )


@dataclass(slots=True)
class ItemResult:
    """Resultado del servidor para un ítem del lote."""

    index: int
    status: str
    external_id: str | None = None
    product_id: str | None = None
    offer_id: int | None = None
    errors: list[dict[str, Any]] = field(default_factory=list)

    @classmethod
    def from_payload(cls, payload: Mapping[str, Any]) -> "ItemResult":
        return cls(
            index=int(payload.get("index") or 0),
            status=str(payload.get("status") or ""),
            external_id=payload.get("externalId"),
            product_id=payload.get("productId"),
            offer_id=payload.get("offerId"),
            errors=list(payload.get("errors") or []),
        )


@dataclass(slots=True)
class BatchResult:
    """Respuesta de un envío; ``failed`` no implica error HTTP."""

    run_id: int
    received: int = 0
    processed: int = 0
    failed: int = 0
    results: list[ItemResult] = field(default_factory=list)

    @classmethod
    def from_payload(cls, payload: Mapping[str, Any]) -> "BatchResult":
        return cls(
            run_id=int(payload.get("runId") or 0),
            received=int(payload.get("received") or 0),
            processed=int(payload.get("processed") or 0),
            failed=int(payload.get("failed") or 0),
            results=[
                ItemResult.from_payload(item)
                for item in payload.get("results") or ()
            ],
        )

    def merge(self, other: "BatchResult", offset: int) -> None:
        self.received += other.received
        self.processed += other.processed
        self.failed += other.failed
        for result in other.results:
            result.index += offset
            self.results.append(result)


class IngestionClient:
    """Cliente asíncrono y sin estado de Ingestion API."""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        timeout: float = 15.0,
        max_retries: int = 3,
        backoff: float = 1.0,
        batch_size: int = 100,
        transport: Any = None,
    ):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.max_retries = max(0, max_retries)
        self.backoff = backoff
        self.batch_size = max(1, min(100, batch_size))
        self._transport = transport or _post_json

    # -- construcción --------------------------------------------------------

    @classmethod
    def from_config(cls) -> "IngestionClient | None":
        """Devuelve un cliente si hay URL y token utilizables; si no, ``None``.

        Sin token no se puede autenticar. Un token de menos de 32 caracteres lo
        ignora ``authz``, así que se descarta aquí con un aviso en vez de
        provocar un 401 en cada lote.
        """
        token = cfg.INGESTION_API_TOKEN.strip()
        if not token:
            _log.info("ingestion disabled: INGESTION_API_TOKEN is not set")
            return None
        parsed = urlparse(cfg.INGESTION_API_URL)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            _log.warning(
                "ingestion disabled: INGESTION_API_URL must be an absolute "
                "http(s) URL, got %r", cfg.INGESTION_API_URL)
            return None
        if len(token) < MIN_TOKEN_LENGTH:
            _log.warning(
                "ingestion disabled: token has %d chars, the backend requires "
                "at least %d", len(token), MIN_TOKEN_LENGTH)
            return None
        return cls(
            cfg.INGESTION_API_URL,
            token,
            timeout=cfg.INGESTION_TIMEOUT,
            max_retries=cfg.INGESTION_MAX_RETRIES,
            backoff=cfg.INGESTION_BACKOFF_BASE,
            batch_size=cfg.INGESTION_BATCH_SIZE,
        )

    # -- operaciones ---------------------------------------------------------

    async def open_run(
        self,
        source_name: str,
        base_url: str | None,
        store_id: int,
        scraper_type: str = "http",
    ) -> RunInfo:
        payload = {
            "source": {"name": source_name, "baseUrl": base_url or ""},
            "storeId": store_id,
            "scraperType": scraper_type,
        }
        status, body, _ = await self._call("/ingestion/runs", payload)
        if status != 201:
            raise IngestionError(
                f"esperaba 201 al abrir la ejecución, recibió {status}",
                status=status, code=_code(body))
        return RunInfo.from_payload(body)

    async def send_items(
        self,
        run_id: int,
        items: list[dict[str, Any]],
    ) -> BatchResult:
        """Envía de 1 a ``batch_size`` ítems; si llegan más, los trocea."""
        if not items:
            raise ValueError("send_items needs at least one item")

        merged = BatchResult(run_id=run_id)
        for offset in range(0, len(items), self.batch_size):
            chunk = items[offset:offset + self.batch_size]
            status, body, _ = await self._call(
                f"/ingestion/runs/{run_id}/items", {"items": chunk})
            if status != 200:
                raise IngestionError(
                    f"esperaba 200 al enviar el lote, recibió {status}",
                    status=status, code=_code(body))
            merged.merge(BatchResult.from_payload(body), offset)
        return merged

    async def finish_run(
        self,
        run_id: int,
        status: str,
        error_message: str | None = None,
    ) -> RunInfo:
        if status not in {"success", "failed"}:
            raise ValueError(f"invalid ingestion status {status!r}")
        payload: dict[str, Any] = {"status": status}
        if error_message:
            payload["errorMessage"] = error_message[:2000]
        http_status, body, _ = await self._call(
            f"/ingestion/runs/{run_id}/finish", payload)
        if http_status != 200:
            raise IngestionError(
                f"esperaba 200 al cerrar la ejecución, recibió {http_status}",
                status=http_status, code=_code(body))
        return RunInfo.from_payload(body)

    # -- transporte ----------------------------------------------------------

    async def _call(
        self, path: str, payload: dict[str, Any],
    ) -> tuple[int, dict[str, Any], Mapping[str, str]]:
        url = f"{self.base_url}{path}"
        attempt = 0
        while True:
            try:
                return await asyncio.to_thread(
                    self._transport, url, payload, self.token, self.timeout)
            except IngestionError as exc:
                if not exc.retriable or attempt >= self.max_retries:
                    raise
                delay = self.backoff * (2 ** attempt)
                _log.warning(
                    "%s failed (%s); retrying in %.1fs (%d/%d)",
                    path, exc, delay, attempt + 1, self.max_retries)
                await asyncio.sleep(delay)
                attempt += 1


def _post_json(
    url: str,
    payload: dict[str, Any],
    token: str,
    timeout: float,
) -> tuple[int, dict[str, Any], Mapping[str, str]]:
    """POST síncrono; se ejecuta en un hilo para no bloquear el event loop."""
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = Request(
        url,
        data=data,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "SOLOServis-crawler/1.0",
        },
    )
    try:
        with urlopen(request, timeout=timeout) as response:
            return response.status, _decode(response.read()), response.headers
    except HTTPError as exc:
        body = _decode(exc.read())
        raise IngestionError(
            str(body.get("detail") or body.get("title") or exc.reason),
            status=exc.code,
            code=_code(body),
            field_errors=tuple(body.get("errors") or ()),
            retriable=exc.code in TRANSIENT_STATUS,
        ) from None
    except (URLError, TimeoutError, OSError,
            http.client.HTTPException) as exc:
        reason = getattr(exc, "reason", exc)
        raise IngestionError(
            f"no se pudo conectar con {url}: {reason}",
            retriable=True,
        ) from None
    except ValueError as exc:  # URL malformada: reintentar no la arregla
        raise IngestionError(
            f"la URL de Ingestion no es válida ({url}): {exc}",
            retriable=False,
        ) from None


def _decode(raw: bytes) -> dict[str, Any]:
    if not raw:
        return {}
    try:
        parsed = json.loads(raw.decode("utf-8", errors="replace"))
    except json.JSONDecodeError:
        return {"detail": raw.decode("utf-8", errors="replace")[:500]}
    return parsed if isinstance(parsed, dict) else {"detail": str(parsed)[:500]}


def _code(body: Mapping[str, Any]) -> str | None:
    code = body.get("code")
    return str(code) if code else None
