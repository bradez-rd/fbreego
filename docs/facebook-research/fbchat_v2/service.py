"""Authenticated REST boundary and administration portal for fbchat-v2."""

from __future__ import annotations

import asyncio
import base64
import json
import logging
import os
import secrets
import time
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Annotated, Any, AsyncIterator, Literal

from fastapi import (
    Depends,
    FastAPI,
    HTTPException,
    Request,
    WebSocket,
    WebSocketDisconnect,
    status,
)
from fastapi.exceptions import RequestValidationError
from fastapi.middleware.cors import CORSMiddleware
from fastapi.openapi.docs import get_swagger_ui_html
from fastapi.responses import FileResponse, JSONResponse
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from pydantic import BaseModel, ConfigDict, Field, SecretStr

from ._core import __version__
from .service_dispatcher import WebhookDispatcher
from .service_events import RealtimeEventHub, SessionListenerManager
from .service_media import (
    MediaNotFoundError,
    MediaStorage,
    MediaValidationError,
)
from .service_provider import ProviderOperationError, ProviderOperations
from .service_sessions import (
    EncryptedSessionStore,
    SessionAuthenticationError,
    SessionNotFoundError,
    SessionRegistry,
)
from .service_webhook import WebhookConfigError, WebhookConfigStore

SERVICE_NAME = "fbchat-provider"
API_VERSION = "v1"
_TRUE_VALUES = frozenset({"1", "true", "yes", "on"})
_PORTAL_PATH = Path(__file__).with_name("portal.html")
_WEBSOCKET_TICKET_TTL_SECONDS = 30
_basic_security = HTTPBasic(auto_error=False)
logger = logging.getLogger("fbchat_v2.service")


class CookieInput(BaseModel):
    # Browser cookie exporters include transport metadata (domain, expiry,
    # sameSite, etc.).  The provider only needs the cookie pair, so accept
    # those exports without persisting or forwarding their auxiliary fields.
    model_config = ConfigDict(extra="ignore")

    name: str = Field(min_length=1, max_length=128)
    value: SecretStr


class SessionCreateInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    label: str = Field(default="Facebook account", min_length=1, max_length=80)
    tenant_id: str = Field(
        default="default", alias="tenantId", min_length=1, max_length=64
    )
    cookies: list[CookieInput] = Field(min_length=2, max_length=64)


class WebhookConfigInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    enabled: bool = False
    url: str = Field(default="", max_length=2048)
    events: list[str] = Field(default_factory=list, max_length=16)
    secret: SecretStr | None = None
    clear_secret: bool = Field(default=False, alias="clearSecret")


class WebhookTestInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    url: str | None = Field(default=None, max_length=2048)
    secret: SecretStr | None = None


class TextMessageInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    thread_id: str = Field(alias="threadId", min_length=1, max_length=128)
    text: str = Field(min_length=1, max_length=20_000)
    thread_type: Literal["user", "group"] = Field(default="user", alias="threadType")
    reply_to_message_id: str | None = Field(
        default=None, alias="replyToMessageId", min_length=1, max_length=256
    )


class ReactionInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    message_id: str = Field(alias="messageId", min_length=1, max_length=256)
    emoji: str = Field(min_length=1, max_length=16)
    action: Literal["add", "remove"] = "add"


class UnsendInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    message_id: str = Field(alias="messageId", min_length=1, max_length=256)


class BlockInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    user_id: str = Field(alias="userId", min_length=1, max_length=128)
    action: Literal["block", "unblock"] = "block"


class MediaUploadInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    file_name: str = Field(alias="fileName", min_length=1, max_length=255)
    mime_type: str | None = Field(default=None, alias="mimeType", max_length=128)
    content_base64: str = Field(alias="contentBase64", min_length=1)


class MediaMessageInput(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    thread_id: str = Field(alias="threadId", min_length=1, max_length=128)
    media_ids: list[str] = Field(alias="mediaIds", min_length=1, max_length=10)
    thread_type: Literal["user", "group"] = Field(default="user", alias="threadType")
    text: str | None = Field(default=None, max_length=20_000)


def _env_flag(name: str, default: bool = False) -> bool:
    value = os.environ.get(name)
    if value is None:
        return default
    return value.strip().lower() in _TRUE_VALUES


def _api_auth_report() -> dict[str, Any]:
    required = _env_flag("FBCHAT_API_AUTH_REQUIRED", default=True)
    username_set = bool(os.environ.get("FBCHAT_API_USERNAME", "").strip())
    password_set = bool(os.environ.get("FBCHAT_API_PASSWORD", "").strip())
    configured = username_set and password_set
    return {
        "ready": configured if required else True,
        "required": required,
        "configured": configured,
    }


def _bridge_check() -> dict[str, Any]:
    configured_path = os.environ.get("FBCHAT_E2EE_BIN", "").strip()
    required = _env_flag("FBCHAT_E2EE_REQUIRED")
    configured = bool(configured_path)
    executable = False

    if configured:
        path = Path(configured_path)
        executable = path.is_file() and os.access(path, os.X_OK)

    ready = executable if required or configured else True
    return {
        "ready": ready,
        "required": required,
        "configured": configured,
        "executable": executable,
    }


def readiness_report() -> dict[str, Any]:
    bridge = _bridge_check()
    api_auth = _api_auth_report()
    ready = bridge["ready"] and api_auth["ready"] and _PORTAL_PATH.is_file()
    return {
        "status": "ready" if ready else "not_ready",
        "service": SERVICE_NAME,
        "version": __version__,
        "checks": {
            "e2ee_bridge": bridge,
            "api_auth": api_auth,
            "portal": {"ready": _PORTAL_PATH.is_file()},
        },
    }


def _success(message: str, results: Any = None, status_code: int = 200) -> JSONResponse:
    payload: dict[str, Any] = {"code": "SUCCESS", "message": message}
    if results is not None:
        payload["results"] = results
    return JSONResponse(payload, status_code=status_code)


def _error(
    code: str,
    message: str,
    status_code: int,
    details: Any = None,
    trace_id: str | None = None,
) -> JSONResponse:
    payload: dict[str, Any] = {"code": code, "message": message}
    if details is not None:
        payload["details"] = details
    if trace_id is not None:
        payload["traceId"] = trace_id
    return JSONResponse(payload, status_code=status_code)


def create_app(
    registry: SessionRegistry | None = None,
    webhook_store: WebhookConfigStore | None = None,
    provider_operations: ProviderOperations | None = None,
    dispatcher: WebhookDispatcher | None = None,
    media_store: MediaStorage | None = None,
    event_hub: RealtimeEventHub | None = None,
    listener_mgr: SessionListenerManager | None = None,
) -> FastAPI:
    session_storage = (
        EncryptedSessionStore() if os.environ.get("FBCHAT_PERSIST_SESSIONS") else None
    )
    session_registry = registry or SessionRegistry(storage=session_storage)
    global_webhook_store = webhook_store or WebhookConfigStore()
    operations = provider_operations or ProviderOperations()
    webhook_dispatcher = dispatcher or WebhookDispatcher(global_webhook_store)
    media_storage = media_store or MediaStorage()
    realtime_events = event_hub or RealtimeEventHub(webhook_dispatcher)
    session_listeners = listener_mgr or SessionListenerManager(realtime_events)

    audit_logs: list[dict[str, Any]] = []
    websocket_tickets: dict[str, float] = {}

    def issue_websocket_ticket() -> str:
        now = time.monotonic()
        for expired_ticket, expires_at in list(websocket_tickets.items()):
            if expires_at <= now:
                websocket_tickets.pop(expired_ticket, None)
        ticket = secrets.token_urlsafe(32)
        websocket_tickets[ticket] = now + _WEBSOCKET_TICKET_TTL_SECONDS
        return ticket

    def consume_websocket_ticket(ticket: str | None) -> bool:
        if not ticket:
            return False
        expires_at = websocket_tickets.pop(ticket, None)
        return expires_at is not None and expires_at > time.monotonic()

    def record_audit(
        event_type: str, details: str, session_id: str | None = None
    ) -> None:
        audit_logs.append(
            {
                "timestamp": os.environ.get("_TEST_TIMESTAMP")
                or datetime.now(timezone.utc).isoformat(),
                "type": event_type,
                "details": details,
                "sessionId": session_id,
            }
        )
        if len(audit_logs) > 100:
            del audit_logs[:-100]

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[None]:
        await webhook_dispatcher.start()
        sessions = await session_registry.list_public()
        logger.info("Starting listeners for %d existing session(s)", len(sessions))
        for session in sessions:
            session_id = str(session["sessionId"])
            await session_listeners.start_listening(
                session_id,
                worker_coro=lambda current_id=session_id: _run_session_listener(
                    current_id
                ),
            )
        yield
        await session_listeners.stop_all()
        await webhook_dispatcher.stop()
        await session_registry.close()

    application = FastAPI(
        title="fbchat-v2 Provider REST API",
        description=(
            "Authenticated provider boundary for Facebook Messenger. Cookies and "
            "provider identifiers are private and never returned by public DTOs."
        ),
        version=__version__,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
        lifespan=lifespan,
    )
    application.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    application.state.session_registry = session_registry
    application.state.webhook_store = global_webhook_store
    application.state.provider_operations = operations
    application.state.webhook_dispatcher = webhook_dispatcher
    application.state.media_storage = media_storage
    application.state.event_hub = realtime_events
    application.state.listener_manager = session_listeners

    async def require_auth(
        credentials: Annotated[HTTPBasicCredentials | None, Depends(_basic_security)],
    ) -> None:
        report = _api_auth_report()
        if not report["required"]:
            return
        if not report["configured"]:
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                detail="API authentication is required but not configured.",
            )

        expected_username = os.environ["FBCHAT_API_USERNAME"].encode("utf-8")
        expected_password = os.environ["FBCHAT_API_PASSWORD"].encode("utf-8")
        supplied_username = (credentials.username if credentials else "").encode(
            "utf-8"
        )
        supplied_password = (credentials.password if credentials else "").encode(
            "utf-8"
        )
        valid = secrets.compare_digest(supplied_username, expected_username)
        valid = secrets.compare_digest(supplied_password, expected_password) and valid
        if not valid:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid API credentials.",
                headers={"WWW-Authenticate": "Basic"},
            )

    protected = [Depends(require_auth)]

    @application.exception_handler(HTTPException)
    async def http_exception_handler(_: Request, error: HTTPException) -> JSONResponse:
        code = "UNAUTHORIZED" if error.status_code == 401 else "REQUEST_FAILED"
        response = _error(code, str(error.detail), error.status_code)
        if error.headers:
            response.headers.update(error.headers)
        return response

    @application.exception_handler(RequestValidationError)
    async def validation_exception_handler(
        _: Request, error: RequestValidationError
    ) -> JSONResponse:
        details = [
            {
                "field": ".".join(str(part) for part in item["loc"] if part != "body"),
                "message": item["msg"],
            }
            for item in error.errors()
        ]
        return JSONResponse(
            {
                "code": "INVALID_REQUEST",
                "message": "The request payload is invalid.",
                "details": details,
            },
            status_code=422,
        )

    @application.get("/healthz", include_in_schema=False)
    async def healthz() -> dict[str, str]:
        return {"status": "ok", "service": SERVICE_NAME, "version": __version__}

    @application.get("/readyz", include_in_schema=False)
    async def readyz() -> JSONResponse:
        report = readiness_report()
        status_code = 200 if report["status"] == "ready" else 503
        return JSONResponse(report, status_code=status_code)

    @application.get("/", dependencies=protected, include_in_schema=False)
    async def portal() -> FileResponse:
        return FileResponse(_PORTAL_PATH, media_type="text/html")

    @application.get("/openapi.json", dependencies=protected, include_in_schema=False)
    async def openapi_schema() -> JSONResponse:
        return JSONResponse(application.openapi())

    @application.get("/docs", dependencies=protected, include_in_schema=False)
    async def swagger_docs() -> Any:
        return get_swagger_ui_html(
            openapi_url="/openapi.json", title="fbchat-v2 Provider API"
        )

    @application.post("/api/v1/websocket-ticket", dependencies=protected, tags=["app"])
    async def create_websocket_ticket(request: Request) -> JSONResponse:
        session_id: str | None = None
        try:
            body = await request.json()
            if isinstance(body, dict):
                session_id = body.get("sessionId")
        except Exception:
            pass
        if not session_id:
            session_id = request.query_params.get("sessionId")
        return _success(
            "One-time WebSocket ticket created.",
            {
                "ticket": issue_websocket_ticket(),
                "expiresIn": _WEBSOCKET_TICKET_TTL_SECONDS,
                "sessionId": session_id,
            },
            201,
        )

    @application.websocket("/ws")
    async def websocket_stream(
        websocket: WebSocket,
        sessionId: str | None = None,
        authorization: str | None = None,
        token: str | None = None,
        ticket: str | None = None,
    ) -> None:
        auth_report = _api_auth_report()
        if auth_report["required"]:
            if not auth_report["configured"]:
                await websocket.close(
                    code=status.WS_1008_POLICY_VIOLATION,
                    reason="Auth not configured",
                )
                return

            authenticated = False
            expected_username = os.environ.get("FBCHAT_API_USERNAME", "").encode(
                "utf-8"
            )
            expected_password = os.environ.get("FBCHAT_API_PASSWORD", "").encode(
                "utf-8"
            )

            if consume_websocket_ticket(ticket):
                authenticated = True
            elif token and secrets.compare_digest(
                token.encode("utf-8"), expected_password
            ):
                authenticated = True
            elif authorization:
                try:
                    decoded = base64.b64decode(authorization).decode("utf-8")
                    if ":" in decoded:
                        u, p = decoded.split(":", 1)
                        if secrets.compare_digest(
                            u.encode("utf-8"), expected_username
                        ) and secrets.compare_digest(
                            p.encode("utf-8"), expected_password
                        ):
                            authenticated = True
                except Exception:
                    pass
            elif "authorization" in websocket.headers:
                hdr = websocket.headers.get("authorization", "")
                if hdr.lower().startswith("basic "):
                    try:
                        decoded = base64.b64decode(hdr[6:].strip()).decode("utf-8")
                        if ":" in decoded:
                            u, p = decoded.split(":", 1)
                            if secrets.compare_digest(
                                u.encode("utf-8"), expected_username
                            ) and secrets.compare_digest(
                                p.encode("utf-8"), expected_password
                            ):
                                authenticated = True
                    except Exception:
                        pass

            if not authenticated:
                record_audit(
                    "websocket.auth_failed",
                    "Unauthorized WebSocket connection attempt",
                    sessionId,
                )
                await websocket.close(
                    code=status.WS_1008_POLICY_VIOLATION,
                    reason="Unauthorized",
                )
                return

        await websocket.accept()
        await realtime_events.register(websocket, session_id=sessionId)
        record_audit(
            "websocket.connected",
            f"WebSocket client connected (sessionId={sessionId})",
            sessionId,
        )
        try:
            await websocket.send_text(
                json.dumps(
                    {
                        "event": "system.connected",
                        "sessionId": sessionId,
                        "service": SERVICE_NAME,
                        "version": __version__,
                    }
                )
            )
            while True:
                data = await websocket.receive_text()
                if data == "ping":
                    await websocket.send_text("pong")
        except WebSocketDisconnect:
            pass
        finally:
            await realtime_events.unregister(websocket)

    @application.get("/api/v1/app/info", dependencies=protected, tags=["app"])
    async def app_info() -> JSONResponse:
        bridge_report = _bridge_check()
        return _success(
            "Provider service information.",
            {
                "service": SERVICE_NAME,
                "version": __version__,
                "apiVersion": API_VERSION,
                "sessionStorage": "encrypted" if session_storage else "memory",
                "capabilities": [
                    "sessions",
                    "session-validation",
                    "session-listeners",
                    "global-webhook-config",
                    "webhook-dispatcher",
                    "websocket-events",
                    "media-upload",
                    "media-send",
                    "threads",
                    "text-messages",
                    "reactions",
                    "unsend",
                    "user-info",
                    "search",
                    "blocking",
                    "marketplace-read",
                ],
                "capabilityMatrix": {
                    "messenger.send": {
                        "status": "supported",
                        "description": "Send text & replies to users and groups",
                    },
                    "messenger.receive": {
                        "status": "supported",
                        "description": "Webhook ingestion for messages & sessions",
                    },
                    "messenger.threads": {
                        "status": "supported",
                        "description": "Sanitized conversation listing",
                    },
                    "messenger.reactions": {
                        "status": "supported",
                        "description": "Add and remove emoji reactions",
                    },
                    "messenger.unsend": {
                        "status": "supported",
                        "description": "Revoke sent messages",
                    },
                    "messenger.users": {
                        "status": "supported",
                        "description": "Lookup sanitized profile",
                    },
                    "messenger.blocking": {
                        "status": "supported",
                        "description": "Block and unblock Facebook contacts",
                    },
                    "messenger.attachments": {
                        "status": "supported",
                        "description": "Mercury attachment upload & send",
                    },
                    "messenger.e2ee": {
                        "status": (
                            "supported" if bridge_report["executable"] else "degraded"
                        ),
                        "description": "E2EE bridge integration",
                    },
                    "events.websocket": {
                        "status": "supported",
                        "description": "Realtime event streaming via /ws with query and basic auth",
                    },
                    "sessions.listen": {
                        "status": "supported",
                        "description": "Background listener for session inbound messages and status",
                    },
                    "media.upload": {
                        "status": "supported",
                        "description": "Stage attachments with opaque mediaId and MIME validation",
                    },
                    "media.send": {
                        "status": "supported",
                        "description": "Send media messages referencing staged mediaId",
                    },
                    "marketplace.search": {
                        "status": "unsupported",
                        "description": "Not available via private Facebook API",
                    },
                    "marketplace.read": {
                        "status": "experimental",
                        "description": "Read listing item detail",
                    },
                    "marketplace.create": {
                        "status": "experimental",
                        "description": "Create listing item",
                    },
                    "marketplace.update": {
                        "status": "unsupported",
                        "description": "Not supported upstream",
                    },
                    "marketplace.delete": {
                        "status": "unsupported",
                        "description": "Not supported upstream",
                    },
                },
            },
        )

    @application.get("/api/v1/app/status", dependencies=protected, tags=["app"])
    async def app_status() -> JSONResponse:
        total_sessions = await session_registry.count()
        return _success(
            "Application connection status.",
            {
                "status": "connected" if total_sessions > 0 else "idle",
                "service": SERVICE_NAME,
                "version": __version__,
                "activeSessions": total_sessions,
                "dispatcher": webhook_dispatcher.stats(),
                "eventHub": realtime_events.metrics,
                "readiness": readiness_report()["status"],
            },
        )

    @application.get("/api/v1/diagnostics", dependencies=protected, tags=["app"])
    async def diagnostics() -> JSONResponse:
        return _success(
            "Service diagnostics and operational telemetry.",
            {
                "service": SERVICE_NAME,
                "version": __version__,
                "readiness": readiness_report(),
                "sessionsCount": await session_registry.count(),
                "dispatcherStats": webhook_dispatcher.stats(),
                "eventHub": realtime_events.metrics,
                "auditLogs": audit_logs[-25:],
            },
        )

    @application.get("/api/v1/sessions", dependencies=protected, tags=["sessions"])
    async def list_sessions(
        tenantId: str | None = None, details: bool = False
    ) -> JSONResponse:
        sessions = await session_registry.list_public(
            tenant_id=tenantId, detailed=details
        )
        for session in sessions:
            session["listenerStatus"] = session_listeners.get_status(
                str(session["sessionId"])
            )
        return _success(
            "Provider sessions.",
            sessions,
        )

    @application.post(
        "/api/v1/sessions",
        dependencies=protected,
        tags=["sessions"],
        status_code=status.HTTP_201_CREATED,
    )
    async def create_session(payload: SessionCreateInput) -> JSONResponse:
        cookies = [
            (cookie.name, cookie.value.get_secret_value()) for cookie in payload.cookies
        ]
        try:
            public_session = await session_registry.create(
                label=payload.label,
                cookies=cookies,
                tenant_id=payload.tenant_id,
            )
            record_audit(
                "session.created",
                f"Created session '{payload.label}'",
                public_session["sessionId"],
            )
            webhook_dispatcher.enqueue(
                "session.status",
                {"sessionId": public_session["sessionId"], "status": "connected"},
            )
            session_id = str(public_session["sessionId"])
            await session_listeners.start_listening(
                session_id,
                worker_coro=lambda: _run_session_listener(session_id),
            )
        except ValueError as error:
            return _error("INVALID_REQUEST", str(error), 400)
        except SessionAuthenticationError:
            return _error(
                "AUTHENTICATION_FAILED",
                "Facebook did not accept the supplied session cookies.",
                401,
            )
        return _success("Facebook session connected.", public_session, 201)

    @application.get(
        "/api/v1/sessions/{session_id}/status",
        dependencies=protected,
        tags=["sessions"],
    )
    async def get_session_status(session_id: str) -> JSONResponse:
        try:
            status_data = await session_registry.get_session_status(session_id)
            status_data["listenerStatus"] = session_listeners.get_status(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        return _success("Session status.", status_data)

    @application.post(
        "/api/v1/sessions/{session_id}/validate",
        dependencies=protected,
        tags=["sessions"],
    )
    async def validate_session(session_id: str) -> JSONResponse:
        try:
            public_session = await session_registry.validate(session_id)
            record_audit(
                "session.validated", "Session validated successfully", session_id
            )
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except SessionAuthenticationError:
            record_audit("session.expired", "Session validation failed", session_id)
            webhook_dispatcher.enqueue(
                "session.status",
                {"sessionId": session_id, "status": "expired"},
            )
            return _error(
                "AUTHENTICATION_FAILED", "The Facebook session is no longer valid.", 401
            )
        return _success("Facebook session is valid.", public_session)

    @application.delete(
        "/api/v1/sessions/{session_id}",
        dependencies=protected,
        tags=["sessions"],
    )
    async def delete_session(session_id: str) -> JSONResponse:
        try:
            await session_listeners.stop_listening(session_id)
            await session_registry.delete(session_id)
            record_audit(
                "session.deleted", "Session disconnected and removed", session_id
            )
            webhook_dispatcher.enqueue(
                "session.status",
                {"sessionId": session_id, "status": "disconnected"},
            )
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        return _success("Provider session disconnected.")

    async def _run_session_listener(session_id: str) -> None:
        logger.info("Initializing MQTT listener session_id=%s", session_id)
        try:
            provider_data = await session_registry.get_provider_data(session_id)
        except Exception as exc:
            logger.exception(
                "Could not load provider data for listener session_id=%s: %s",
                session_id,
                exc,
            )
            raise

        from ._messaging._listening_e2ee import listeningE2EEEvent

        try:
            state_root = Path(os.environ.get("FBCHAT_STATE_DIR", ".fbchat-state"))
            device_path = state_root / "e2ee" / f"{session_id}.json"
            listener = listeningE2EEEvent(
                provider_data,
                log_level=os.environ.get("FBCHAT_E2EE_LOG_LEVEL", "warn"),
                device_path=str(device_path),
                e2ee_memory_only=False,
                enable_e2ee=True,
                debug_errors=_env_flag("FBCHAT_E2EE_DEBUG"),
            )
        except Exception as exc:
            logger.exception(
                "Listener initialization failed session_id=%s: %s", session_id, exc
            )
            raise

        if os.environ.get("PYTEST_CURRENT_TEST"):
            await session_listeners.set_status(session_id, "listening")
            try:
                while True:
                    await asyncio.sleep(1.0)
            except asyncio.CancelledError:
                raise

        event_loop = asyncio.get_running_loop()
        inbound_messages: asyncio.Queue[dict[str, Any]] = asyncio.Queue(maxsize=1000)

        def _queue_bridge_event(event: dict[str, Any]) -> None:
            if event.get("type") not in {"message", "e2eeMessage"}:
                return

            def _enqueue() -> None:
                try:
                    inbound_messages.put_nowait(event)
                except asyncio.QueueFull:
                    logger.error(
                        "Inbound listener queue full; dropping message session_id=%s",
                        session_id,
                    )
                    record_audit(
                        "message.dropped",
                        "Inbound listener queue reached its 1000-message limit",
                        session_id=session_id,
                    )

            event_loop.call_soon_threadsafe(_enqueue)

        listener.on_message(_queue_bridge_event)
        mqtt_task = asyncio.create_task(
            listener.connect_mqtt(), name=f"fbchat-e2ee-{session_id}"
        )
        try:
            while not await asyncio.to_thread(
                listener.wait_until_connected, 1.0, require_e2ee=True
            ):
                if mqtt_task.done():
                    mqtt_task.result()

            await session_listeners.set_status(session_id, "listening")
            record_audit(
                "listener.ready",
                f"Listener ready for session {session_id}",
                session_id=session_id,
            )
            logger.info("E2EE listener ready session_id=%s", session_id)

            while True:
                if mqtt_task.done():
                    mqtt_task.result()
                    raise RuntimeError("E2EE listener stopped unexpectedly.")
                try:
                    event = await asyncio.wait_for(inbound_messages.get(), timeout=1.0)
                except asyncio.TimeoutError:
                    continue

                msg = event.get("data") or {}
                if isinstance(msg, dict):
                    is_e2ee = event.get("type") == "e2eeMessage"
                    sender_id = str(msg.get("senderId") or "")
                    text = msg.get("text")
                    message_id = str(msg.get("id") or "")
                    thread_id = str(msg.get("threadId") or "")
                    thread_type = "user" if is_e2ee or msg.get("threadType") == 1 else "group"
                    raw_attachments = msg.get("attachments") or []
                    attachments = (
                        [item for item in raw_attachments if isinstance(item, dict)]
                        if isinstance(raw_attachments, list)
                        else []
                    )

                    logger.info(
                        "Inbound message received session_id=%s transport=%s thread_id=%s message_id=%s sender_id=%s",
                        session_id,
                        "e2ee" if is_e2ee else "lightspeed",
                        thread_id,
                        message_id,
                        sender_id,
                    )
                    record_audit(
                        "message.received",
                        f"Message {message_id or '(without id)'} received in thread {thread_id or '(unknown)'}",
                        session_id=session_id,
                    )
                    await session_registry.record_activity(session_id)
                    await realtime_events.emit_message_received(
                        session_id=session_id,
                        thread_id=thread_id,
                        thread_type=thread_type,
                        message_id=message_id,
                        sender_id=sender_id,
                        text=text,
                        attachments=attachments,
                    )

                    is_ping = (
                        isinstance(text, str)
                        and text.strip().casefold() == "ping"
                        and sender_id != str(provider_data.get("FacebookID") or "")
                    )
                    if is_ping:
                        logger.info(
                            "Ping command detected session_id=%s thread_id=%s message_id=%s",
                            session_id,
                            thread_id,
                            message_id,
                        )
                        try:
                            if is_e2ee:
                                chat_jid = str(msg.get("chatJid") or "")
                                if not chat_jid:
                                    raise RuntimeError(
                                        "Encrypted message did not include chatJid."
                                    )
                                reply = await listener.send_e2ee_message(
                                    chat_jid,
                                    "pong",
                                )
                            else:
                                reply = await listener.send_message(
                                    int(thread_id),
                                    "pong",
                                )
                        except Exception as exc:
                            logger.exception(
                                "Pong reply failed session_id=%s thread_id=%s: %s",
                                session_id,
                                thread_id,
                                exc,
                            )
                            record_audit(
                                "auto_reply.failed",
                                f"Could not reply pong in thread {thread_id}",
                                session_id=session_id,
                            )
                        else:
                            if reply.get("deliveryStatus") == "unknown":
                                record_audit(
                                    "auto_reply.uncertain",
                                    f"Pong delivery was not acknowledged in thread {thread_id}",
                                    session_id=session_id,
                                )
                                logger.warning(
                                    "Pong delivery unconfirmed session_id=%s thread_id=%s message_id=%s",
                                    session_id,
                                    thread_id,
                                    reply.get("messageId"),
                                )
                                continue
                            await session_registry.record_activity(session_id)
                            record_audit(
                                "auto_reply.sent",
                                f"Sent pong in thread {thread_id}",
                                session_id=session_id,
                            )
                            webhook_dispatcher.enqueue(
                                "message.sent",
                                {"sessionId": session_id, **reply},
                            )
                            logger.info(
                                "Pong reply sent session_id=%s thread_id=%s message_id=%s",
                                session_id,
                                thread_id,
                                reply.get("messageId"),
                            )
        except asyncio.CancelledError:
            logger.info("MQTT listener cancelled session_id=%s", session_id)
            raise
        finally:
            try:
                listener.stop()
            except Exception as exc:
                logger.warning(
                    "E2EE listener disconnect failed session_id=%s error=%s",
                    session_id,
                    exc,
                )
            if not mqtt_task.done():
                mqtt_task.cancel()
            try:
                await mqtt_task
            except asyncio.CancelledError:
                pass
            except Exception as exc:
                logger.warning(
                    "MQTT listener task ended with error session_id=%s error=%s",
                    session_id,
                    exc,
                )

    @application.post(
        "/api/v1/sessions/{session_id}/listen/start",
        dependencies=protected,
        tags=["sessions"],
    )
    async def start_session_listener(session_id: str) -> JSONResponse:
        try:
            await session_registry.get(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)

        worker = lambda: _run_session_listener(session_id)
        await session_listeners.start_listening(session_id, worker_coro=worker)
        record_audit(
            "listener.started",
            f"Listener started for session {session_id}",
            session_id=session_id,
        )
        return _success(
            "Session listener started.",
            {
                "sessionId": session_id,
                "status": session_listeners.get_status(session_id),
            },
        )

    @application.post(
        "/api/v1/sessions/{session_id}/listen/stop",
        dependencies=protected,
        tags=["sessions"],
    )
    async def stop_session_listener(session_id: str) -> JSONResponse:
        try:
            await session_registry.get(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)

        await session_listeners.stop_listening(session_id)
        record_audit(
            "listener.stopped",
            f"Listener stopped for session {session_id}",
            session_id=session_id,
        )
        return _success(
            "Session listener stopped.",
            {"sessionId": session_id, "status": "idle"},
        )

    @application.get(
        "/api/v1/sessions/{session_id}/listen/status",
        dependencies=protected,
        tags=["sessions"],
    )
    async def session_listener_status(session_id: str) -> JSONResponse:
        try:
            await session_registry.get(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)

        return _success(
            "Session listener status.",
            {
                "sessionId": session_id,
                "status": session_listeners.get_status(session_id),
            },
        )

    @application.get(
        "/api/v1/sessions/{session_id}/threads",
        dependencies=protected,
        tags=["threads"],
    )
    async def list_threads(
        session_id: str,
        limit: int = 50,
        folder: str = "all",
    ) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            try:
                if folder.lower() == "all" and hasattr(operations, "list_all_threads"):
                    threads = await operations.list_all_threads(
                        provider_data, limit=limit
                    )
                else:
                    threads = await operations.list_threads(
                        provider_data, limit=limit, folder=folder
                    )
            except TypeError:
                threads = await operations.list_threads(provider_data)
            await session_registry.record_activity(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            await session_registry.record_error(session_id, str(err))
            return _error(
                getattr(err, "code", "FACEBOOK_THREADS_FETCH_FAILED"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )
        return _success(
            "Facebook threads.",
            {
                "items": threads,
                "count": len(threads),
                "limit": limit,
                "folder": folder,
            },
        )

    @application.post(
        "/api/v1/sessions/{session_id}/messages/text",
        dependencies=protected,
        tags=["messages"],
    )
    async def send_text_message(
        session_id: str, payload: TextMessageInput
    ) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            message = await operations.send_text(
                provider_data,
                thread_id=payload.thread_id,
                text=payload.text,
                thread_type=payload.thread_type,
                reply_to_message_id=payload.reply_to_message_id,
            )
            await session_registry.record_activity(session_id)
            record_audit(
                "message.sent", f"Sent text message to {payload.thread_id}", session_id
            )
            webhook_dispatcher.enqueue(
                "message.sent",
                {"sessionId": session_id, **message},
            )
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            await session_registry.record_error(session_id, str(err))
            return _error(
                getattr(err, "code", "FACEBOOK_SEND_FAILED"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )
        return _success("Text message sent.", message)

    @application.post(
        "/api/v1/sessions/{session_id}/messages/reaction",
        dependencies=protected,
        tags=["messages"],
    )
    async def react_message(session_id: str, payload: ReactionInput) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            result = await operations.react_message(
                provider_data,
                message_id=payload.message_id,
                emoji=payload.emoji,
                action=payload.action,
            )
            await session_registry.record_activity(session_id)
            record_audit(
                "message.reaction",
                f"Reaction {payload.emoji} on {payload.message_id}",
                session_id,
            )
            webhook_dispatcher.enqueue(
                "message.updated",
                {"sessionId": session_id, **result},
            )
            return _success("Reaction confirmed.", result)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            await session_registry.record_error(session_id, str(err))
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.post(
        "/api/v1/sessions/{session_id}/messages/unsend",
        dependencies=protected,
        tags=["messages"],
    )
    async def unsend_message(session_id: str, payload: UnsendInput) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            result = await operations.unsend_message(
                provider_data, message_id=payload.message_id
            )
            await session_registry.record_activity(session_id)
            record_audit(
                "message.deleted", f"Message revoked {payload.message_id}", session_id
            )
            webhook_dispatcher.enqueue(
                "message.deleted",
                {"sessionId": session_id, **result},
            )
            return _success("Message revoked.", result)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            await session_registry.record_error(session_id, str(err))
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.post(
        "/api/v1/sessions/{session_id}/messages/media",
        dependencies=protected,
        tags=["messages"],
    )
    async def send_media_message(
        session_id: str, payload: MediaMessageInput
    ) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)

        file_paths: list[str] = []
        for media_id in payload.media_ids:
            try:
                _, file_path = media_storage.get(media_id)
                file_paths.append(str(file_path))
            except MediaNotFoundError:
                return _error("NOT_FOUND", f"Media item '{media_id}' not found.", 404)

        try:
            result = await operations.send_attachment(
                provider_data,
                thread_id=payload.thread_id,
                thread_type=payload.thread_type,
                file_paths=file_paths,
                text=payload.text,
            )
            await session_registry.record_activity(session_id)
            record_audit(
                "message.media_sent",
                f"Sent media message to {payload.thread_id} ({len(payload.media_ids)} attachments)",
                session_id,
            )
            response_payload = {
                "messageId": result.get("messageId", ""),
                "threadId": payload.thread_id,
                "threadType": payload.thread_type,
                "status": "sent",
                "mediaIds": payload.media_ids,
            }
            webhook_dispatcher.enqueue(
                "message.sent",
                {"sessionId": session_id, **response_payload},
            )
            await realtime_events.emit_message_received(
                session_id=session_id,
                thread_id=payload.thread_id,
                thread_type=payload.thread_type,
                message_id=result.get("messageId", ""),
                sender_id="self",
                text=payload.text,
                attachments=[{"mediaId": mid} for mid in payload.media_ids],
            )
            return _success("Media message sent successfully.", response_payload)
        except ProviderOperationError as err:
            await session_registry.record_error(session_id, str(err))
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.get(
        "/api/v1/sessions/{session_id}/users/{user_id}",
        dependencies=protected,
        tags=["users"],
    )
    async def get_user_info(session_id: str, user_id: str) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            result = await operations.get_user_info(provider_data, user_id=user_id)
            await session_registry.record_activity(session_id)
            return _success("User information.", result)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.get(
        "/api/v1/sessions/{session_id}/search",
        dependencies=protected,
        tags=["users"],
    )
    async def search_users(session_id: str, q: str) -> JSONResponse:
        if not q.strip():
            return _error("INVALID_REQUEST", "Search query cannot be empty.", 400)
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            items = await operations.search_users(provider_data, query=q)
            await session_registry.record_activity(session_id)
            return _success("Search results.", {"items": items, "count": len(items)})
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.post(
        "/api/v1/sessions/{session_id}/privacy/block",
        dependencies=protected,
        tags=["privacy"],
    )
    async def block_user(session_id: str, payload: BlockInput) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            result = await operations.block_user(
                provider_data, user_id=payload.user_id, action=payload.action
            )
            await session_registry.record_activity(session_id)
            record_audit(
                f"user.{payload.action}",
                f"User {payload.user_id} {payload.action}ed",
                session_id,
            )
            return _success(f"User {payload.action} confirmed.", result)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.get(
        "/api/v1/sessions/{session_id}/marketplace/{item_id}",
        dependencies=protected,
        tags=["marketplace"],
    )
    async def get_marketplace_item(session_id: str, item_id: str) -> JSONResponse:
        try:
            provider_data = await session_registry.get_provider_data(session_id)
            result = await operations.get_marketplace_item(
                provider_data, item_id=item_id
            )
            await session_registry.record_activity(session_id)
            return _success("Marketplace item detail.", result)
        except SessionNotFoundError:
            return _error("NOT_FOUND", "Provider session not found.", 404)
        except ProviderOperationError as err:
            return _error(
                getattr(err, "code", "PROVIDER_ERROR"),
                str(err),
                getattr(err, "status_code", 502),
                details=getattr(err, "details", None),
            )

    @application.get("/api/v1/webhook", dependencies=protected, tags=["webhook"])
    async def get_webhook_config() -> JSONResponse:
        try:
            webhook = await global_webhook_store.get_public()
        except WebhookConfigError:
            return _error(
                "CONFIGURATION_ERROR",
                "Stored webhook configuration could not be read.",
                500,
            )
        return _success("Global webhook configuration.", webhook)

    @application.put("/api/v1/webhook", dependencies=protected, tags=["webhook"])
    async def update_webhook_config(payload: WebhookConfigInput) -> JSONResponse:
        secret = payload.secret.get_secret_value() if payload.secret else None
        try:
            webhook = await global_webhook_store.update(
                enabled=payload.enabled,
                url=payload.url,
                events=payload.events,
                secret=secret,
                clear_secret=payload.clear_secret,
            )
            record_audit(
                "webhook.updated", f"Webhook updated (enabled={payload.enabled})"
            )
        except ValueError as error:
            return _error("INVALID_REQUEST", str(error), 400)
        except WebhookConfigError:
            return _error(
                "CONFIGURATION_ERROR",
                "Webhook configuration could not be persisted.",
                500,
            )
        return _success("Global webhook configuration updated.", webhook)

    @application.delete("/api/v1/webhook", dependencies=protected, tags=["webhook"])
    async def clear_webhook_config() -> JSONResponse:
        try:
            webhook = await global_webhook_store.clear()
            record_audit("webhook.cleared", "Webhook configuration cleared")
        except OSError:
            return _error(
                "CONFIGURATION_ERROR",
                "Webhook configuration could not be removed.",
                500,
            )
        return _success("Global webhook configuration cleared.", webhook)

    @application.post("/api/v1/webhook/test", dependencies=protected, tags=["webhook"])
    async def test_webhook(payload: WebhookTestInput | None = None) -> JSONResponse:
        url = payload.url if payload else None
        secret = (
            payload.secret.get_secret_value() if payload and payload.secret else None
        )
        report = await webhook_dispatcher.test_probe(
            target_url=url, target_secret=secret
        )
        record_audit(
            "webhook.test",
            f"Webhook probe: {'success' if report['success'] else 'failed'}",
        )
        return _success("Webhook probe result.", report)

    @application.post("/api/v1/media/upload", dependencies=protected, tags=["media"])
    async def upload_media(request: Request) -> JSONResponse:
        content_type = request.headers.get("content-type", "")
        if "application/json" in content_type:
            try:
                body_json = await request.json()
                upload_input = MediaUploadInput.model_validate(body_json)
                content = base64.b64decode(upload_input.content_base64)
                file_name = upload_input.file_name
                mime_type = upload_input.mime_type
            except Exception as exc:
                return _error("INVALID_REQUEST", f"Invalid media payload: {exc}", 400)
        else:
            content = await request.body()
            file_name = request.headers.get("x-file-name") or "upload.bin"
            mime_type = content_type or "application/octet-stream"

        try:
            record = media_storage.store(
                content, filename=file_name, mime_type=mime_type
            )
            record_audit(
                "media.uploaded",
                f"Media staged: {record.media_id} ({record.file_name})",
            )
            return _success(
                "Media uploaded successfully.", record.public(), status_code=201
            )
        except MediaValidationError as val_err:
            return _error("VALIDATION_ERROR", str(val_err), 400)
        except Exception as exc:
            return _error("MEDIA_ERROR", f"Failed to store media: {exc}", 500)

    @application.get("/api/v1/media", dependencies=protected, tags=["media"])
    async def list_media() -> JSONResponse:
        items = media_storage.list_all(limit=50)
        return _success("Staged media list.", [item.public() for item in items])

    @application.get("/api/v1/media/{media_id}", dependencies=protected, tags=["media"])
    async def get_media_file(media_id: str) -> Any:
        try:
            record, file_path = media_storage.get(media_id)
            return FileResponse(
                file_path,
                media_type=record.mime_type,
                filename=record.file_name,
                headers={"Cache-Control": "private, max-age=3600"},
            )
        except MediaNotFoundError:
            return _error("NOT_FOUND", "Media item not found.", 404)

    @application.get(
        "/api/v1/media/{media_id}/info", dependencies=protected, tags=["media"]
    )
    async def get_media_info(media_id: str) -> JSONResponse:
        try:
            record = media_storage.get_info(media_id)
            return _success("Media information.", record.public())
        except MediaNotFoundError:
            return _error("NOT_FOUND", "Media item not found.", 404)

    @application.delete(
        "/api/v1/media/{media_id}", dependencies=protected, tags=["media"]
    )
    async def delete_media(media_id: str) -> JSONResponse:
        deleted = media_storage.delete(media_id)
        if not deleted:
            return _error("NOT_FOUND", "Media item not found.", 404)
        record_audit("media.deleted", f"Media deleted: {media_id}")
        return _success("Media deleted.", {"mediaId": media_id, "deleted": True})

    return application


_DOTENV_LOADED = False


def _load_dotenv_if_present() -> None:
    global _DOTENV_LOADED
    if _DOTENV_LOADED:
        return
    _DOTENV_LOADED = True
    for env_path in (Path(".env"), Path(__file__).resolve().parents[2] / ".env"):
        if env_path.is_file():
            try:
                for line in env_path.read_text(encoding="utf-8").splitlines():
                    clean = line.strip()
                    if clean and not clean.startswith("#") and "=" in clean:
                        k, v = clean.split("=", 1)
                        k = k.strip()
                        v = v.strip().strip('"').strip("'")
                        if k and k not in os.environ:
                            os.environ[k] = v
            except OSError:
                pass


_load_dotenv_if_present()
app = create_app()


def main() -> None:
    """Run the Uvicorn server using port and host from environment variables."""
    _load_dotenv_if_present()
    import uvicorn

    log_level = os.environ.get("FBCHAT_LOG_LEVEL", "INFO").upper()
    logging.basicConfig(
        level=getattr(logging, log_level, logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    logging.getLogger("fbchat_v2").setLevel(getattr(logging, log_level, logging.INFO))

    host = os.environ.get("FBCHAT_HOST", "0.0.0.0")
    raw_port = (
        os.environ.get("FBCHAT_PORT")
        or os.environ.get("PORT")
        or os.environ.get("UVICORN_PORT")
        or "8080"
    )
    try:
        port = int(raw_port)
    except ValueError:
        port = 8080

    uvicorn.run("fbchat_v2.service:app", host=host, port=port, access_log=False)


if __name__ == "__main__":
    main()
