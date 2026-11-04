"""Sanitized provider operations exposed by the REST boundary."""

from __future__ import annotations

import json
from pathlib import Path
import re
import time
from typing import Any

import httpx

from ._features._facebook._blocking import func as blocking_func
from ._features._facebook._get_user_info import func as get_user_info_func
from ._features._facebook._marketplace import (
    getInformationProductItemMarketPlace as get_marketplace_item_func,
)
from ._features._facebook._search import func as search_func
from ._features._thread._all_thread_data import func as fetch_thread_data
from ._messaging._attachments import func as upload_attachment_func
from ._messaging._reactions import func as react_func
from ._messaging._send import api as SendAPI
from ._messaging._unsend import func as unsend_func


class ProviderOperationError(RuntimeError):
    """Raised when Facebook cannot complete a REST provider operation."""

    def __init__(
        self,
        message: str,
        code: str = "FACEBOOK_UPSTREAM_ERROR",
        status_code: int = 502,
        details: dict[str, Any] | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.status_code = status_code
        self.details = details or {}


def _thread_id(node: dict[str, Any]) -> str:
    key = node.get("thread_key") or {}
    return str(key.get("thread_fbid") or key.get("other_user_id") or "").strip()


def _sanitize_threads(
    result: dict[str, Any],
    folder_label: str = "inbox",
) -> list[dict[str, Any]]:
    try:
        parsed = json.loads(result["dataGet"])
        o0 = parsed.get("o0") or {}
        if "errors" in o0 and not o0.get("data"):
            summary = (
                o0["errors"][0].get("summary")
                or o0["errors"][0].get("message")
                or "Facebook GraphQL error"
            )
            raise ProviderOperationError(
                f"Facebook error: {summary}",
                code="FACEBOOK_THREADS_FETCH_FAILED",
            )
        nodes = (
            o0.get("data", {})
            .get("viewer", {})
            .get("message_threads", {})
            .get("nodes", [])
        )
    except (KeyError, TypeError, json.JSONDecodeError) as error:
        raise ProviderOperationError(
            "Facebook returned invalid thread data.",
            code="FACEBOOK_THREADS_FETCH_FAILED",
        ) from error

    threads: list[dict[str, Any]] = []
    for node in nodes:
        if not isinstance(node, dict):
            continue
        thread_id = _thread_id(node)
        if not thread_id:
            continue
        name = node.get("name")
        if not name:
            edges = node.get("all_participants", {}).get("edges", [])
            for edge in edges:
                actor = edge.get("node", {}).get("messaging_actor", {})
                if actor.get("name"):
                    name = actor.get("name")
                    if actor.get("id") and str(actor.get("id")) == str(thread_id):
                        break
        # Use the folder value from the node itself if available,
        # otherwise fall back to the label we know we requested.
        raw_folder = str(node.get("folder") or "").strip().lower()
        if raw_folder in ("inbox", "pending", "other", "archived", "archive"):
            effective_folder = "archived" if raw_folder == "archive" else raw_folder
        else:
            effective_folder = "archived" if folder_label in ("archive", "archived") else folder_label
        threads.append(
            {
                "threadId": thread_id,
                "type": str(node.get("thread_type") or "unknown").lower(),
                "name": name,
                "folder": effective_folder,
                "unreadCount": int(node.get("unread_count") or 0),
                "updatedAt": node.get("updated_time_precise"),
                "archived": bool(node.get("has_viewer_archived", False)),
                "marketplace": bool(node.get("marketplace_thread_data")),
            }
        )
    return threads


# Maps user-facing folder names to Facebook GraphQL tag values.
_FOLDER_TAGS: dict[str, list[str]] = {
    "inbox": ["INBOX"],
    "pending": ["PENDING"],
    "archive": ["ARCHIVED"],
    "archived": ["ARCHIVED"],
    "other": ["OTHER"],
}

# Folders queried when the caller asks for "all".
_ALL_FOLDERS = ("inbox", "pending", "other", "archive")


def _clean_ls_arg(a: Any) -> Any:
    if isinstance(a, list) and len(a) == 2 and a[0] == 19:
        return a[1]
    return a


def _extract_ls_calls(node: Any):
    if isinstance(node, list):
        if len(node) >= 2 and node[0] == 5 and isinstance(node[1], str):
            yield (node[1], node[2:])
        for sub in node:
            yield from _extract_ls_calls(sub)


def _find_all_ls_requests(obj: Any):
    if isinstance(obj, dict):
        if "lightspeed_web_request" in obj:
            yield obj["lightspeed_web_request"]
        for v in obj.values():
            yield from _find_all_ls_requests(v)
    elif isinstance(obj, list):
        for item in obj:
            yield from _find_all_ls_requests(item)


def _parse_lightspeed_data_from_html(
    html: str,
) -> tuple[list[dict[str, Any]], dict[str, dict[str, Any]]]:
    """Parse LightSpeed thread updates and contacts from Facebook HTML."""
    scripts = re.findall(
        r'<script type="application/json"[^>]*>(.*?)</script>', html, re.DOTALL
    )
    contacts: dict[str, dict[str, Any]] = {}
    threads: list[dict[str, Any]] = []

    for s in scripts:
        if "lightspeed_web_request" not in s:
            continue
        try:
            data = json.loads(s)
        except Exception:
            continue

        for ls in _find_all_ls_requests(data):
            payload_str = ls.get("payload")
            if not payload_str:
                continue
            try:
                payload = json.loads(payload_str)
            except Exception:
                continue

            steps = payload.get("step") or []
            calls = list(_extract_ls_calls(steps))

            # Pass 1: Parse contacts
            for name, args in calls:
                if name == "verifyContactRowExists":
                    cleaned = [_clean_ls_arg(a) for a in args]
                    if len(cleaned) >= 4:
                        uid = str(cleaned[0] or "").strip()
                        cname = str(cleaned[3] or "").strip() if cleaned[3] else None
                        avatar = str(cleaned[2] or "").strip() if cleaned[2] else None
                        if uid and cname:
                            contacts[uid] = {
                                "id": uid,
                                "name": cname,
                                "avatar": avatar,
                            }

            # Pass 2: Parse threads
            for name, args in calls:
                if name in ("updateOrInsertThread", "deleteThenInsertThread"):
                    cleaned = [_clean_ls_arg(a) for a in args]
                    if len(cleaned) > 10:
                        tid = str(cleaned[7] or "").strip()
                        if not tid or not tid.isdigit():
                            continue
                        updated_at = str(cleaned[0] or "")
                        folder = str(cleaned[10] or "inbox").lower()
                        unread_count = 0
                        if len(cleaned) > 13:
                            raw_unread = cleaned[13]
                            if (
                                isinstance(raw_unread, (int, str))
                                and str(raw_unread).isdigit()
                            ):
                                unread_count = int(raw_unread)
                        ttype_code = str(cleaned[9] or "")
                        ttype = "group" if ttype_code in ("15", "2") else "one_to_one"
                        snippet = None
                        if (
                            len(cleaned) > 40
                            and isinstance(cleaned[40], str)
                            and cleaned[40].strip()
                        ):
                            snippet = cleaned[40].strip()
                        creator_id = None
                        if len(cleaned) > 21:
                            raw_cid = str(cleaned[21] or "").strip()
                            if raw_cid.isdigit():
                                creator_id = raw_cid

                        contact_obj = contacts.get(creator_id or "") or contacts.get(tid)
                        resolved_name = (
                            contact_obj["name"]
                            if contact_obj and contact_obj.get("name")
                            else (snippet or f"Chat {tid}")
                        )

                        threads.append(
                            {
                                "threadId": tid,
                                "type": ttype,
                                "name": resolved_name,
                                "folder": folder,
                                "unreadCount": unread_count,
                                "updatedAt": updated_at,
                                "archived": folder in ("archive", "archived"),
                                "marketplace": False,
                                "snippet": snippet,
                            }
                        )

    return threads, contacts


async def fetch_lightspeed_data(
    provider_data: dict[str, Any],
    *,
    force_refresh: bool = False,
) -> tuple[list[dict[str, Any]], dict[str, dict[str, Any]]]:
    now = time.time()
    last_fetched = float(provider_data.get("_ls_fetched_at") or 0)
    cached_threads = provider_data.get("_ls_threads")
    cached_contacts = provider_data.get("_ls_contacts")
    if (
        not force_refresh
        and (now - last_fetched < 15)
        and cached_threads
        and cached_contacts is not None
    ):
        return cached_threads, cached_contacts

    cookie_str = provider_data.get("cookieFacebook") or ""
    if not cookie_str:
        return [], {}
    try:
        from ._core._session import _build_home_request, send_get_request_async

        req = _build_home_request(cookie_str)
        res = await send_get_request_async(req)
        if res.is_success:
            threads, contacts = _parse_lightspeed_data_from_html(res.text)
            if threads or contacts:
                provider_data["_ls_threads"] = threads
                provider_data["_ls_contacts"] = contacts
                provider_data["_ls_fetched_at"] = now
            return threads, contacts
    except Exception:
        pass
    return [], {}


class ProviderOperations:
    """Async adapter over the fbchat-v2 feature modules."""

    async def list_threads(
        self,
        provider_data: dict[str, Any],
        limit: int = 50,
        folder: str = "all",
    ) -> list[dict[str, Any]]:
        folder_clean = (folder or "all").strip().lower()
        if folder_clean == "all":
            return await self.list_all_threads(provider_data, limit=limit)

        all_merged = await self.list_all_threads(provider_data, limit=limit)
        target_folder = "archived" if folder_clean in ("archive", "archived") else folder_clean
        return [
            t for t in all_merged
            if (t.get("folder") or "inbox").lower() == target_folder
        ]

    async def list_all_threads(
        self,
        provider_data: dict[str, Any],
        limit: int = 50,
    ) -> list[dict[str, Any]]:
        """Query GraphQL folders and LightSpeed sync data concurrently and merge."""
        import asyncio

        async def _fetch_folder(folder: str) -> list[dict[str, Any]]:
            tags = _FOLDER_TAGS.get(folder, ["INBOX"])
            try:
                result = await fetch_thread_data(
                    provider_data, limit=limit, tags=tags
                )
                return _sanitize_threads(result, folder_label=folder)
            except PermissionError as err:
                raise ProviderOperationError(
                    str(err),
                    code="FACEBOOK_SESSION_EXPIRED",
                    status_code=401,
                ) from err
            except ProviderOperationError:
                return []
            except (httpx.HTTPError, KeyError, TypeError, ValueError):
                return []

        async def _fetch_ls() -> list[dict[str, Any]]:
            try:
                ls_threads, _ = await fetch_lightspeed_data(
                    provider_data, force_refresh=True
                )
                return ls_threads
            except Exception:
                return []

        folder_tasks = [_fetch_folder(f) for f in _ALL_FOLDERS]
        results = await asyncio.gather(*folder_tasks, _fetch_ls())

        merged_map: dict[str, dict[str, Any]] = {}
        # 1. Add GraphQL batch results
        for batch in results[:-1]:
            for thread in batch:
                merged_map[thread["threadId"]] = thread

        # 2. Merge LightSpeed results (newer timestamps & modern E2EE/hybrid threads)
        ls_batch = results[-1]
        for thread in ls_batch:
            tid = thread["threadId"]
            if tid in merged_map:
                existing = merged_map[tid]
                ls_ts = int(thread.get("updatedAt") or 0)
                ex_ts = int(existing.get("updatedAt") or 0)
                if ls_ts >= ex_ts:
                    existing["updatedAt"] = thread["updatedAt"]
                    existing["folder"] = thread["folder"]
                if thread.get("name") and (
                    not existing.get("name")
                    or existing.get("name").startswith("Chat ")
                ):
                    existing["name"] = thread["name"]
                if thread.get("snippet") and not existing.get("snippet"):
                    existing["snippet"] = thread["snippet"]
            else:
                merged_map[tid] = thread

        all_threads = list(merged_map.values())
        all_threads.sort(
            key=lambda t: int(t.get("updatedAt") or "0"), reverse=True
        )
        group_ids = {t["threadId"] for t in all_threads if t.get("type") == "group"}
        if group_ids:
            existing_groups = set(provider_data.get("_group_thread_ids") or ())
            provider_data["_group_thread_ids"] = list(existing_groups | group_ids)
        return all_threads

    async def send_text(
        self,
        provider_data: dict[str, Any],
        *,
        thread_id: str,
        text: str,
        thread_type: str = "user",
        reply_to_message_id: str | None = None,
    ) -> dict[str, Any]:
        effective_type = thread_type
        if effective_type == "user":
            known_groups = set(provider_data.get("_group_thread_ids") or ())
            if str(thread_id) in known_groups:
                effective_type = "group"
        try:
            result = await SendAPI().send(
                provider_data,
                text,
                thread_id,
                typeChat="user" if effective_type == "user" else None,
                replyMessage=bool(reply_to_message_id),
                messageID=reply_to_message_id,
            )
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError(
                "Facebook message delivery failed.",
                code="FACEBOOK_SEND_FAILED",
                details={"error": str(error), "threadId": thread_id},
            ) from error

        if result.get("success") != 1:
            payload = result.get("payload") or {}
            err_desc = (
                payload.get("error-description")
                or result.get("errorDescription")
                or "Facebook did not confirm message delivery."
            )
            err_code = payload.get("error-code") or result.get("error")
            code_str = (
                "FACEBOOK_SESSION_EXPIRED"
                if err_code in (1357004, 1357001)
                else "FACEBOOK_SEND_FAILED"
            )
            raise ProviderOperationError(
                f"Facebook send error: {err_desc}",
                code=code_str,
                status_code=401 if code_str == "FACEBOOK_SESSION_EXPIRED" else 502,
                details={
                    "facebookErrorCode": err_code,
                    "facebookErrorDescription": err_desc,
                    "threadId": thread_id,
                },
            )
        payload = result.get("payload") or {}
        message_id = str(payload.get("messageID") or "").strip()
        if not message_id:
            raise ProviderOperationError(
                "Facebook did not return a message ID.",
                code="FACEBOOK_SEND_FAILED",
            )
        return {
            "messageId": message_id,
            "threadId": thread_id,
            "status": "sent",
            "timestamp": payload.get("timestamp"),
        }

    async def react_message(
        self,
        provider_data: dict[str, Any],
        *,
        message_id: str,
        emoji: str,
        action: str = "add",
    ) -> dict[str, Any]:
        try:
            act = (
                "ADD_REACTION"
                if action.lower() in {"add", "add_reaction"}
                else "REMOVE_REACTION"
            )
            response = await react_func(provider_data, act, message_id, emoji)
            if hasattr(response, "status_code") and response.status_code >= 400:
                raise ProviderOperationError(
                    f"Facebook reaction failed with status {response.status_code}."
                )
            return {
                "messageId": message_id,
                "emoji": emoji,
                "action": "add" if act == "ADD_REACTION" else "remove",
                "status": "confirmed",
            }
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError("Facebook reaction failed.") from error

    async def unsend_message(
        self, provider_data: dict[str, Any], *, message_id: str
    ) -> dict[str, Any]:
        try:
            result = await unsend_func(message_id, provider_data)
            if result.get("error"):
                raise ProviderOperationError(
                    str(result.get("messages") or "Facebook rejected unsend.")
                )
            return {"messageId": message_id, "status": "revoked"}
        except ProviderOperationError:
            raise
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError("Facebook message revoke failed.") from error

    async def get_user_info(
        self, provider_data: dict[str, Any], *, user_id: str
    ) -> dict[str, Any]:
        target_uid = str(user_id).strip()

        # 1. Self lookup
        self_id = str(provider_data.get("FacebookID") or "").strip()
        if target_uid and target_uid == self_id:
            return {
                "userId": self_id,
                "name": provider_data.get("name") or "Bryan Aguiar",
                "firstName": (provider_data.get("name") or "Bryan").split(" ")[0],
                "username": None,
                "gender": "Unknown",
                "isNonFriend": False,
            }

        # 2. Upstream Facebook chat user_info
        try:
            result = await get_user_info_func(provider_data, target_uid)
            if not result.get("error"):
                return {
                    "userId": str(result.get("idUser") or target_uid),
                    "name": result.get("nameUser"),
                    "firstName": result.get("firstName"),
                    "username": result.get("Username"),
                    "gender": result.get("genderUser"),
                    "isNonFriend": bool(result.get("chatWithUSerIsNonFriend")),
                }
        except (httpx.HTTPError, KeyError, TypeError, ValueError):
            pass

        # 3. LightSpeed contacts and thread participants
        try:
            threads, contacts = await fetch_lightspeed_data(provider_data)
            if target_uid in contacts:
                c = contacts[target_uid]
                cname = c.get("name") or "Facebook User"
                return {
                    "userId": target_uid,
                    "name": cname,
                    "firstName": cname.split(" ")[0] if cname else None,
                    "username": None,
                    "gender": "Unknown",
                    "isNonFriend": True,
                }
            for t in threads:
                if t.get("threadId") == target_uid or t.get("creatorId") == target_uid:
                    tname = t.get("name")
                    if tname and not tname.startswith("Chat "):
                        return {
                            "userId": target_uid,
                            "name": tname,
                            "firstName": tname.split(" ")[0] if tname else None,
                            "username": None,
                            "gender": "Unknown",
                            "isNonFriend": True,
                        }
        except Exception:
            pass

        raise ProviderOperationError(
            f"User {target_uid} not found.", code="USER_NOT_FOUND", status_code=404
        )

    async def search_users(
        self, provider_data: dict[str, Any], *, query: str
    ) -> list[dict[str, Any]]:
        found: list[dict[str, Any]] = []
        try:
            result = await search_func(provider_data, query)
            if not result.get("error"):
                items = result.get("searchResultsDict") or []
                for item in items:
                    uid = str(item.get("id") or "").strip()
                    if uid:
                        found.append(
                            {
                                "userId": uid,
                                "name": item.get("name"),
                                "url": item.get("url"),
                            }
                        )
        except Exception:
            pass

        try:
            threads, contacts = await fetch_lightspeed_data(provider_data)
            q_lower = query.lower()
            existing_ids = {u["userId"] for u in found}
            for cid, cinfo in contacts.items():
                if cid not in existing_ids:
                    cname = cinfo.get("name") or ""
                    if q_lower in cname.lower() or q_lower == cid:
                        found.append(
                            {
                                "userId": cid,
                                "name": cname,
                                "url": f"https://www.facebook.com/{cid}",
                            }
                        )
                        existing_ids.add(cid)
            for t in threads:
                tid = str(t.get("threadId") or "").strip()
                tname = str(t.get("name") or "").strip()
                if tid and tid not in existing_ids and not tname.startswith("Chat "):
                    if q_lower in tname.lower() or q_lower == tid:
                        found.append(
                            {
                                "userId": tid,
                                "name": tname,
                                "url": f"https://www.facebook.com/{tid}",
                            }
                        )
                        existing_ids.add(tid)
        except Exception:
            pass

        return found

    async def block_user(
        self, provider_data: dict[str, Any], *, user_id: str, action: str = "block"
    ) -> dict[str, Any]:
        try:
            clean_action = "block" if action.lower() == "block" else "unblock"
            result = await blocking_func(provider_data, user_id, clean_action)
            if result.get("error"):
                raise ProviderOperationError(
                    str(result.get("messages") or "Block operation failed.")
                )
            return {"userId": user_id, "action": clean_action, "status": "confirmed"}
        except ProviderOperationError:
            raise
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError("Facebook block operation failed.") from error

    async def get_marketplace_item(
        self, provider_data: dict[str, Any], *, item_id: str
    ) -> dict[str, Any]:
        try:
            result = await get_marketplace_item_func(provider_data, item_id)
            if result.get("error"):
                raise ProviderOperationError(
                    str(result.get("messages") or "Item not found."),
                    code="ITEM_NOT_FOUND",
                    status_code=404,
                )
            payload = result.get("payload") or {}
            return {
                "itemId": str(payload.get("idProduct") or item_id),
                "name": payload.get("nameProduct"),
                "price": payload.get("priceProduct"),
                "currency": payload.get("currencyProduct"),
                "description": payload.get("decriptionProduct"),
                "url": payload.get("urlProduct"),
                "sellerId": payload.get("idSeller"),
                "status": "experimental",
            }
        except ProviderOperationError:
            raise
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError(
                "Facebook marketplace lookup failed."
            ) from error

    async def send_attachment(
        self,
        provider_data: dict[str, Any],
        *,
        thread_id: str,
        file_path: str | None = None,
        file_paths: list[str] | None = None,
        thread_type: str = "user",
        text: str = "",
    ) -> dict[str, Any]:
        target_paths = [
            Path(p) for p in (file_paths or ([file_path] if file_path else []))
        ]
        if not target_paths:
            raise ProviderOperationError("Attachment file not provided.")
        for path in target_paths:
            if not path.is_file():
                raise ProviderOperationError(f"Attachment file does not exist: {path}")

        effective_type = thread_type
        if effective_type == "user":
            known_groups = set(provider_data.get("_group_thread_ids") or ())
            if str(thread_id) in known_groups:
                effective_type = "group"

        try:
            uploaded = await upload_attachment_func(
                [str(p) for p in target_paths], provider_data
            )
            if not uploaded or not uploaded.get("attachmentID"):
                raise ProviderOperationError("Facebook attachment upload failed.")

            att_id = uploaded["attachmentID"]
            att_type = uploaded.get("typeAttachment") or "file"

            result = await SendAPI().send(
                provider_data,
                text,
                thread_id,
                typeAttachment=att_type,
                attachmentID=att_id,
                typeChat="user" if effective_type == "user" else None,
            )
        except ProviderOperationError:
            raise
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            raise ProviderOperationError(
                "Facebook attachment delivery failed."
            ) from error

        if result.get("success") != 1:
            raise ProviderOperationError("Facebook did not confirm message delivery.")
        payload = result.get("payload") or {}
        message_id = str(payload.get("messageID") or "").strip()
        return {
            "messageId": message_id,
            "threadId": thread_id,
            "status": "sent",
            "attachmentType": att_type,
            "timestamp": payload.get("timestamp"),
        }
