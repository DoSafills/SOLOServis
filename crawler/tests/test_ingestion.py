import asyncio
import json
import socket
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

import config as cfg
from ingestion import (
    BatchResult,
    IngestionClient,
    IngestionError,
    RunInfo,
)

TOKEN = "t" * 40


class _Handler(BaseHTTPRequestHandler):
    def do_POST(self):  # noqa: N802 (nombre que impone http.server)
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length)
        body = json.loads(raw) if raw else {}
        record = {
            "path": self.path,
            "authorization": self.headers.get("Authorization"),
            "body": body,
        }
        self.server.requests.append(record)
        status, payload = self.server.responder(record)
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):  # silenciar el log del servidor de pruebas
        pass


class _FakeServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, responder):
        super().__init__(("127.0.0.1", 0), _Handler)
        self.responder = responder
        self.requests: list[dict] = []


@pytest.fixture()
def fake_api():
    responses: list[tuple[int, dict]] = []
    server = _FakeServer(lambda request: responses.pop(0))
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server, responses
    finally:
        server.shutdown()
        server.server_close()


def _client(server, **kwargs) -> IngestionClient:
    defaults = dict(
        base_url=f"http://127.0.0.1:{server.server_address[1]}",
        token=TOKEN,
        timeout=5,
        max_retries=0,
        backoff=0,
    )
    defaults.update(kwargs)
    return IngestionClient(**defaults)


RUN_PAYLOAD = {
    "id": 7,
    "source": "Ripley Chile",
    "sourceUrl": "https://simple.ripley.cl",
    "storeId": 4,
    "scraperType": "http",
    "status": "running",
    "recordsFound": 0,
    "recordsProcessed": 0,
    "recordsFailed": 0,
}


class TestOpenRun:
    def test_sends_contract_body_and_auth(self, fake_api):
        server, responses = fake_api
        responses.append((201, RUN_PAYLOAD))

        run = asyncio.run(_client(server).open_run(
            "Ripley Chile", "https://simple.ripley.cl", 4, "http"))

        assert run == RunInfo(
            id=7, source="Ripley Chile", status="running",
            source_url="https://simple.ripley.cl", store_id=4,
            scraper_type="http")
        request = server.requests[0]
        assert request["path"] == "/ingestion/runs"
        assert request["authorization"] == f"Bearer {TOKEN}"
        assert request["body"] == {
            "source": {"name": "Ripley Chile",
                       "baseUrl": "https://simple.ripley.cl"},
            "storeId": 4,
            "scraperType": "http",
        }

    def test_problem_details_become_ingestion_error(self, fake_api):
        server, responses = fake_api
        responses.append((422, {
            "title": "Unprocessable Entity",
            "status": 422,
            "code": "validation_failed",
            "detail": "El lote no cumple las reglas.",
            "errors": [{"field": "storeId", "code": "not_found",
                        "message": "storeId no existe"}],
        }))

        with pytest.raises(IngestionError) as caught:
            asyncio.run(_client(server).open_run("X", None, 99))

        error = caught.value
        assert error.status == 422
        assert error.code == "validation_failed"
        assert error.field_errors == (
            {"field": "storeId", "code": "not_found",
             "message": "storeId no existe"},)
        assert error.retriable is False
        assert "HTTP 422" in str(error)


class TestSendItems:
    def test_batches_of_at_most_100_and_merges_results(self, fake_api):
        server, responses = fake_api
        responses.append((200, {
            "runId": 7, "received": 100, "processed": 100, "failed": 0,
            "results": [],
        }))
        responses.append((200, {
            "runId": 7, "received": 5, "processed": 4, "failed": 1,
            "results": [{"index": 0, "externalId": "x", "status": "failed",
                         "errors": [{"field": "offer.price",
                                     "code": "format",
                                     "message": "precio inválido"}]}],
        }))

        result = asyncio.run(_client(server).send_items(7, [{}] * 105))

        assert len(server.requests) == 2
        assert len(server.requests[0]["body"]["items"]) == 100
        assert len(server.requests[1]["body"]["items"]) == 5
        assert isinstance(result, BatchResult)
        assert (result.received, result.processed, result.failed) == (105, 104, 1)
        # El índice del segundo lote se desplaza para referirse al lote completo.
        assert result.results[0].index == 100

    def test_empty_batch_is_rejected_before_the_network(self, fake_api):
        server, _ = fake_api
        with pytest.raises(ValueError):
            asyncio.run(_client(server).send_items(7, []))
        assert server.requests == []

    def test_server_failure_raises(self, fake_api):
        server, responses = fake_api
        responses.append((409, {
            "code": "conflict", "detail": "La ejecución ya está cerrada."}))

        with pytest.raises(IngestionError) as caught:
            asyncio.run(_client(server).send_items(7, [{}]))

        assert caught.value.status == 409
        assert caught.value.code == "conflict"
        assert caught.value.retriable is False


class TestFinishRun:
    def test_sends_status_and_message(self, fake_api):
        server, responses = fake_api
        responses.append((200, {**RUN_PAYLOAD, "status": "failed"}))

        run = asyncio.run(_client(server).finish_run(
            7, "failed", "ingestion: timeout"))

        assert run.status == "failed"
        assert server.requests[0]["path"] == "/ingestion/runs/7/finish"
        assert server.requests[0]["body"] == {
            "status": "failed", "errorMessage": "ingestion: timeout"}

    def test_invalid_status_is_rejected_locally(self, fake_api):
        server, _ = fake_api
        with pytest.raises(ValueError):
            asyncio.run(_client(server).finish_run(7, "completed"))


class TestRetries:
    def test_retries_transient_status_then_succeeds(self, fake_api):
        server, responses = fake_api
        responses.append((503, {"code": "unavailable", "detail": "try later"}))
        responses.append((201, RUN_PAYLOAD))

        run = asyncio.run(_client(server, max_retries=2).open_run("X", None, 4))

        assert run.id == 7
        assert len(server.requests) == 2

    def test_does_not_retry_client_errors(self, fake_api):
        server, responses = fake_api
        responses.append((400, {"code": "bad_request", "detail": "no"}))

        with pytest.raises(IngestionError):
            asyncio.run(_client(server, max_retries=3).open_run("X", None, 4))

        assert len(server.requests) == 1

    def test_connection_refused_is_retriable(self):
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        # El socket se cerró: el puerto queda libre y la conexión se rechaza.
        client = IngestionClient(
            f"http://127.0.0.1:{port}", TOKEN, max_retries=1, backoff=0)

        with pytest.raises(IngestionError) as caught:
            asyncio.run(client.open_run("X", None, 4))

        assert caught.value.retriable is True
        assert "no se pudo conectar" in str(caught.value)




class TestFromConfig:
    def test_disabled_without_token(self, monkeypatch):
        monkeypatch.setattr(cfg, "INGESTION_API_TOKEN", "")
        assert IngestionClient.from_config() is None

    def test_disabled_with_short_token(self, monkeypatch):
        monkeypatch.setattr(cfg, "INGESTION_API_TOKEN", "short")
        assert IngestionClient.from_config() is None

    def test_disabled_without_url(self, monkeypatch):
        monkeypatch.setattr(cfg, "INGESTION_API_TOKEN", TOKEN)
        monkeypatch.setattr(cfg, "INGESTION_API_URL", "")
        assert IngestionClient.from_config() is None

    def test_disabled_with_malformed_url(self, monkeypatch):
        monkeypatch.setattr(cfg, "INGESTION_API_TOKEN", TOKEN)
        monkeypatch.setattr(cfg, "INGESTION_API_URL", "localhost:8094")
        assert IngestionClient.from_config() is None

    def test_builds_with_valid_token(self, monkeypatch):
        monkeypatch.setattr(cfg, "INGESTION_API_TOKEN", TOKEN)
        monkeypatch.setattr(cfg, "INGESTION_API_URL", "http://api.test")
        client = IngestionClient.from_config()
        assert client is not None
        assert client.base_url == "http://api.test"
        assert client.batch_size == cfg.INGESTION_BATCH_SIZE
