#!/usr/bin/env python3
"""linear.py — CLI de Linear para la terminal.

Sin dependencias externas: sólo la librería estándar de Python 3.9+.

La API key se busca, en este orden:
  1. variable de entorno LINEAR_API_KEY
  2. archivo .env en el directorio actual o en cualquier ancestro
  3. .env en la raíz del repositorio

La key se envía en `Authorization` SIN el prefijo "Bearer": así lo exigen las
personal API keys de Linear. Con "Bearer" la API responde 401.

Uso rápido:
    scripts/linear/linear.py whoami
    scripts/linear/linear.py issues --state Todo --limit 10
    scripts/linear/linear.py mine
    scripts/linear/linear.py person Ricardo
    scripts/linear/linear.py show TAL-233
    scripts/linear/linear.py start TAL-233
    scripts/linear/linear.py comment TAL-233 "listo para revisión"

`--help` en cualquier subcomando muestra sus opciones.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Iterable

API_URL = os.environ.get("LINEAR_API_URL", "https://api.linear.app/graphql")
USER_AGENT = "soloservis-linear-cli/1.0"
TIMEOUT = 60
MAX_RETRIES = 3

PRIORITY_LABELS = {0: "—", 1: "urgent", 2: "high", 3: "medium", 4: "low"}

# Atajos de estado. Se resuelven contra los workflow states reales del equipo.
STATE_ALIASES = {
    "todo": ["Todo"],
    "backlog": ["Backlog"],
    "start": ["In Progress"],
    "started": ["In Progress"],
    "doing": ["In Progress"],
    "progress": ["In Progress"],
    "review": ["In Review"],
    "done": ["Done"],
    "complete": ["Done"],
    "completed": ["Done"],
    "cancel": ["Canceled"],
    "canceled": ["Canceled"],
    "duplicate": ["Duplicate"],
}


# --------------------------------------------------------------------------
# Salida
# --------------------------------------------------------------------------


class Style:
    """Colores ANSI desactivables con NO_COLOR o --no-color."""

    def __init__(self, enabled: bool) -> None:
        self.enabled = enabled

    def _wrap(self, code: str, text: str) -> str:
        return f"\033[{code}m{text}\033[0m" if self.enabled else text

    def bold(self, text: str) -> str:
        return self._wrap("1", text)

    def dim(self, text: str) -> str:
        return self._wrap("2", text)

    def red(self, text: str) -> str:
        return self._wrap("31", text)

    def green(self, text: str) -> str:
        return self._wrap("32", text)

    def yellow(self, text: str) -> str:
        return self._wrap("33", text)

    def cyan(self, text: str) -> str:
        return self._wrap("36", text)


STYLE = Style(sys.stdout.isatty() and not os.environ.get("NO_COLOR"))


def die(message: str, code: int = 1) -> "NoReturn":  # noqa: F821
    print(f"{STYLE.red('error:')} {message}", file=sys.stderr)
    raise SystemExit(code)


def warn(message: str) -> None:
    print(f"{STYLE.yellow('aviso:')} {message}", file=sys.stderr)


# --------------------------------------------------------------------------
# Configuración / credenciales
# --------------------------------------------------------------------------


def _parse_dotenv(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return values
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        key = key.strip()
        if key.startswith("export "):
            key = key[len("export ") :].strip()
        value = value.strip().strip('"').strip("'")
        values[key] = value
    return values


def find_api_key() -> str:
    from_env = os.environ.get("LINEAR_API_KEY", "").strip()
    if from_env:
        return from_env

    starts = [Path.cwd(), Path(__file__).resolve().parent]
    seen: set[Path] = set()
    for start in starts:
        for directory in [start, *start.parents]:
            if directory in seen:
                continue
            seen.add(directory)
            candidate = directory / ".env"
            if candidate.is_file():
                key = _parse_dotenv(candidate).get("LINEAR_API_KEY", "").strip()
                if key:
                    return key

    die(
        "no encontré LINEAR_API_KEY.\n"
        "  export LINEAR_API_KEY=lin_api_...\n"
        "  o define LINEAR_API_KEY en el .env del repositorio."
    )
    raise AssertionError("inalcanzable")


# --------------------------------------------------------------------------
# Cliente GraphQL
# --------------------------------------------------------------------------


class LinearError(Exception):
    """Error de negocio o de transporte contra la API de Linear."""


class Client:
    def __init__(self, api_key: str, url: str = API_URL) -> None:
        self.api_key = api_key
        self.url = url
        self._teams: list[dict[str, Any]] | None = None
        self._users: list[dict[str, Any]] | None = None
        self._states: dict[str, list[dict[str, Any]]] = {}

    # -- transporte --------------------------------------------------------

    def gql(self, query: str, variables: dict[str, Any] | None = None) -> dict[str, Any]:
        payload: dict[str, Any] = {"query": query}
        if variables:
            payload["variables"] = variables
        body = json.dumps(payload).encode("utf-8")
        last_error: Exception | None = None

        for attempt in range(MAX_RETRIES):
            request = urllib.request.Request(
                self.url,
                data=body,
                headers={
                    "Content-Type": "application/json",
                    "Authorization": self.api_key,
                    "User-Agent": USER_AGENT,
                },
                method="POST",
            )
            try:
                with urllib.request.urlopen(request, timeout=TIMEOUT) as response:
                    parsed = json.loads(response.read().decode("utf-8"))
                break
            except urllib.error.HTTPError as exc:
                detail = exc.read().decode("utf-8", errors="replace")[:400]
                if exc.code in (429, 500, 502, 503, 504) and attempt < MAX_RETRIES - 1:
                    time.sleep(2**attempt)
                    last_error = exc
                    continue
                if exc.code == 401:
                    raise LinearError(
                        "401 Unauthorized: la API key es inválida o falta. "
                        "Recuerda que las personal API keys van en Authorization "
                        "sin el prefijo 'Bearer'."
                    ) from exc
                raise LinearError(f"HTTP {exc.code}: {detail}") from exc
            except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
                if attempt < MAX_RETRIES - 1:
                    time.sleep(2**attempt)
                    last_error = exc
                    continue
                raise LinearError(f"no pude contactar la API: {exc}") from exc
        else:  # pragma: no cover - sólo si se agotan los reintentos
            raise LinearError(f"no pude contactar la API: {last_error}")

        errors = parsed.get("errors")
        if errors:
            messages = "; ".join(str(item.get("message", item)) for item in errors)
            raise LinearError(messages)
        return parsed.get("data") or {}

    # -- catálogos cacheados ----------------------------------------------

    def teams(self) -> list[dict[str, Any]]:
        if self._teams is None:
            data = self.gql(
                "query { teams(first: 50) { nodes { id key name issueCount } } }"
            )
            self._teams = data["teams"]["nodes"]
        return self._teams

    def default_team(self) -> str:
        teams = self.teams()
        if not teams:
            raise LinearError("el workspace no tiene equipos")
        if len(teams) == 1:
            return teams[0]["key"]
        keys = ", ".join(team["key"] for team in teams)
        raise LinearError(f"hay varios equipos ({keys}); usa --team")

    def team(self, key_or_name: str | None = None) -> dict[str, Any]:
        target = (key_or_name or self.default_team()).strip().lower()
        for team in self.teams():
            if target in (team["key"].lower(), team["name"].lower()):
                return team
        raise LinearError(f"equipo desconocido: {key_or_name}")

    def users(self) -> list[dict[str, Any]]:
        if self._users is None:
            data = self.gql(
                "query { users(first: 250) { nodes { id name email admin active guest } } }"
            )
            self._users = data["users"]["nodes"]
        return self._users

    def resolve_user(self, needle: str) -> dict[str, Any]:
        """Busca por nombre o email, sin distinguir mayúsculas."""
        target = needle.strip().lower()
        users = self.users()
        exact = [
            user
            for user in users
            if target in (user["name"].lower(), user["email"].lower())
        ]
        if len(exact) == 1:
            return exact[0]
        partial = [
            user
            for user in users
            if target in user["name"].lower() or target in user["email"].lower()
        ]
        if len(partial) == 1:
            return partial[0]
        if not partial:
            raise LinearError(f"no encontré a nadie que coincida con {needle!r}")
        options = ", ".join(f"{u['name']} <{u['email']}>" for u in partial)
        raise LinearError(f"{needle!r} es ambiguo: {options}")

    def states(self, team_key: str) -> list[dict[str, Any]]:
        if team_key not in self._states:
            team = self.team(team_key)
            data = self.gql(
                "query($id: String!) { team(id: $id) { states { nodes "
                "{ id name type position } } } }",
                {"id": team["id"]},
            )
            self._states[team_key] = data["team"]["states"]["nodes"]
        return self._states[team_key]

    def resolve_state(self, team_key: str, wanted: str) -> dict[str, Any]:
        states = self.states(team_key)
        target = wanted.strip().lower()

        for candidate in STATE_ALIASES.get(target, []):
            for state in states:
                if state["name"].lower() == candidate.lower():
                    return state

        for state in states:
            if state["name"].lower() == target:
                return state

        for state in states:
            if target in state["name"].lower():
                return state

        known = ", ".join(state["name"] for state in states)
        raise LinearError(f"estado {wanted!r} no existe en {team_key}; usa uno de: {known}")

    # -- issues ------------------------------------------------------------

    def full_identifier(self, identifier: str) -> str:
        """Acepta `233` o `tal-233` y devuelve `TAL-233`."""
        text = identifier.strip().upper()
        if text.isdigit():
            return f"{self.default_team()}-{text}"
        return text

    def issue(self, identifier: str, *, with_comments: bool = False) -> dict[str, Any]:
        full = self.full_identifier(identifier)
        try:
            data = self.gql(
                "query($id: String!) { issue(id: $id) { id identifier title url priority "
                "createdAt updatedAt description state { id name type } "
                "assignee { id name email } project { id name } "
                "labels { nodes { name } } "
                "comments(first: 20) { nodes { body createdAt user { name } } } } }",
                {"id": full},
            )
        except LinearError as exc:
            if "not found" in str(exc).lower():
                raise LinearError(f"no existe la issue {full}") from exc
            raise
        issue = data.get("issue")
        if not issue:
            raise LinearError(f"no existe la issue {full}")
        if not with_comments:
            issue.pop("comments", None)
        return issue

    def issues(
        self,
        *,
        team: str | None = None,
        state_id: str | None = None,
        assignee_id: str | None = None,
        project: str | None = None,
        text: str | None = None,
        limit: int = 25,
    ) -> list[dict[str, Any]]:
        filters: dict[str, Any] = {}
        if team:
            filters["team"] = {"key": {"eq": team}}
        if state_id:
            filters["state"] = {"id": {"eq": state_id}}
        if assignee_id:
            filters["assignee"] = {"id": {"eq": assignee_id}}
        if project:
            filters["project"] = {"name": {"eq": project}}
        if text:
            filters["or"] = [
                {"title": {"containsIgnoreCase": text}},
                {"description": {"containsIgnoreCase": text}},
            ]

        query = (
            "query($first: Int!, $after: String, $filter: IssueFilter) { "
            "issues(first: $first, after: $after, filter: $filter, orderBy: updatedAt) { "
            "pageInfo { hasNextPage endCursor } "
            "nodes { identifier title priority updatedAt url state { name } "
            "assignee { name } project { name } } } }"
        )

        collected: list[dict[str, Any]] = []
        cursor: str | None = None
        while len(collected) < limit:
            page_size = min(limit - len(collected), 50)
            data = self.gql(
                query,
                {"first": page_size, "after": cursor, "filter": filters or None},
            )
            page = data["issues"]
            collected.extend(page["nodes"])
            if not page["pageInfo"]["hasNextPage"]:
                break
            cursor = page["pageInfo"]["endCursor"]
        return collected[:limit]

    def project_id(self, name: str) -> str:
        data = self.gql("query { projects(first: 100) { nodes { id name } } }")
        projects = data["projects"]["nodes"]
        target = name.strip().lower()
        for project in projects:
            if project["name"].lower() == target:
                return project["id"]
        partial = [p for p in projects if target in p["name"].lower()]
        if len(partial) == 1:
            return partial[0]["id"]
        known = ", ".join(p["name"] for p in projects)
        raise LinearError(f"proyecto {name!r} no encontrado; opciones: {known}")

    def viewer(self) -> dict[str, Any]:
        data = self.gql(
            "query { viewer { id name email admin guest } organization { name urlKey } }"
        )
        return data["viewer"] | {"organization": data["organization"]}

    # -- mutaciones --------------------------------------------------------

    def update_issue(self, issue_id: str, changes: dict[str, Any]) -> dict[str, Any]:
        data = self.gql(
            "mutation($id: String!, $input: IssueUpdateInput!) { "
            "issueUpdate(id: $id, input: $input) { success issue { identifier "
            "state { name } assignee { name } title } } }",
            {"id": issue_id, "input": changes},
        )
        result = data["issueUpdate"]
        if not result.get("success"):
            raise LinearError("Linear rechazó la actualización")
        return result["issue"]

    def add_comment(self, issue_id: str, body: str) -> dict[str, Any]:
        data = self.gql(
            "mutation($input: CommentCreateInput!) { "
            "commentCreate(input: $input) { success comment { id url createdAt } } }",
            {"input": {"issueId": issue_id, "body": body}},
        )
        result = data["commentCreate"]
        if not result.get("success"):
            raise LinearError("Linear rechazó el comentario")
        return result["comment"]

    def create_issue(self, payload: dict[str, Any]) -> dict[str, Any]:
        data = self.gql(
            "mutation($input: IssueCreateInput!) { "
            "issueCreate(input: $input) { success issue { id identifier title url } } }",
            {"input": payload},
        )
        result = data["issueCreate"]
        if not result.get("success"):
            raise LinearError("Linear rechazó la creación")
        return result["issue"]


# --------------------------------------------------------------------------
# Presentación
# --------------------------------------------------------------------------


def short(text: str, width: int) -> str:
    text = " ".join((text or "").split())
    if len(text) <= width:
        return text
    return text[: width - 1] + "…"


def render_table(headers: list[str], rows: list[list[str]]) -> str:
    if not rows:
        return STYLE.dim("(sin resultados)")
    widths = [len(header) for header in headers]
    for row in rows:
        for index, cell in enumerate(row):
            widths[index] = max(widths[index], len(cell))

    out = ["  ".join(header.ljust(widths[index]) for index, header in enumerate(headers))]
    out.append("  ".join("-" * width for width in widths))
    for row in rows:
        out.append("  ".join(cell.ljust(widths[index]) for index, cell in enumerate(row)))
    return "\n".join(line.rstrip() for line in out)


def issue_rows(issues: Iterable[dict[str, Any]]) -> list[list[str]]:
    rows: list[list[str]] = []
    for issue in issues:
        assignee = (issue.get("assignee") or {}).get("name") or "—"
        state = (issue.get("state") or {}).get("name") or "—"
        rows.append(
            [
                issue["identifier"],
                state,
                short(assignee, 18),
                PRIORITY_LABELS.get(issue.get("priority", 0), "—"),
                short(issue.get("title", ""), 58),
            ]
        )
    return rows


def emit(data: Any, as_json: bool) -> None:
    if as_json:
        print(json.dumps(data, indent=2, ensure_ascii=False))


def iso_day(value: str | None) -> str:
    if not value:
        return "—"
    return value[:10]


# --------------------------------------------------------------------------
# Comandos
# --------------------------------------------------------------------------


def cmd_whoami(client: Client, args: argparse.Namespace) -> None:
    viewer = client.viewer()
    if args.json:
        emit(viewer, True)
        return
    org = viewer["organization"]
    print(f"{STYLE.bold(viewer['name'])} <{viewer['email']}>")
    print(f"  admin     {'sí' if viewer['admin'] else 'no'}")
    print(f"  guest     {'sí' if viewer['guest'] else 'no'}")
    print(f"  workspace {org['name']} ({org['urlKey']})")


def cmd_teams(client: Client, args: argparse.Namespace) -> None:
    teams = client.teams()
    if args.json:
        emit(teams, True)
        return
    rows = [
        [team["key"], team["name"], str(team["issueCount"])] for team in teams
    ]
    print(render_table(["KEY", "EQUIPO", "ISSUES"], rows))


def cmd_users(client: Client, args: argparse.Namespace) -> None:
    users = client.users()
    if not args.all:
        users = [user for user in users if user.get("active", True)]
    if args.json:
        emit(users, True)
        return
    rows = [
        [
            short(user["name"], 30),
            short(user["email"], 38),
            "admin" if user["admin"] else "—",
            "active" if user["active"] else "inactive",
        ]
        for user in users
    ]
    print(render_table(["NOMBRE", "EMAIL", "ROL", "ESTADO"], rows))


def cmd_states(client: Client, args: argparse.Namespace) -> None:
    team_key = client.team(args.team)["key"]
    states = sorted(client.states(team_key), key=lambda item: item["position"])
    if args.json:
        emit(states, True)
        return
    rows = [[state["name"], state["type"]] for state in states]
    print(render_table([f"ESTADO ({team_key})", "TIPO"], rows))


def cmd_projects(client: Client, args: argparse.Namespace) -> None:
    data = client.gql(
        "query { projects(first: 50) { nodes { id name state progress "
        "startDate targetDate } } }"
    )
    projects = data["projects"]["nodes"]
    if args.json:
        emit(projects, True)
        return
    rows = [
        [
            short(project["name"], 26),
            project.get("state") or "—",
            f"{(project.get('progress') or 0) * 100:.0f}%",
            iso_day(project.get("targetDate")),
        ]
        for project in projects
    ]
    print(render_table(["PROYECTO", "ESTADO", "AVANCE", "TARGET"], rows))


def cmd_stats(client: Client, args: argparse.Namespace) -> None:
    team_key = client.team(args.team)["key"]
    issues = client.issues(team=team_key, limit=args.limit)
    by_state: dict[str, int] = {}
    by_person: dict[str, int] = {}
    for issue in issues:
        state = (issue.get("state") or {}).get("name") or "—"
        person = (issue.get("assignee") or {}).get("name") or "(sin asignar)"
        by_state[state] = by_state.get(state, 0) + 1
        by_person[person] = by_person.get(person, 0) + 1

    if args.json:
        emit(
            {
                "team": team_key,
                "total": len(issues),
                "by_state": by_state,
                "by_assignee": by_person,
            },
            True,
        )
        return

    order = [
        state["name"]
        for state in sorted(client.states(team_key), key=lambda item: item["position"])
    ]
    ranked = sorted(
        by_state.items(),
        key=lambda item: order.index(item[0]) if item[0] in order else 999,
    )
    print(STYLE.bold(f"{team_key} · {len(issues)} issues"))
    print(render_table(["ESTADO", "ISSUES"], [[name, str(count)] for name, count in ranked]))
    print()
    people = sorted(by_person.items(), key=lambda item: -item[1])
    print(render_table(["ASIGNADO A", "ISSUES"], [[name, str(count)] for name, count in people]))


def _print_issues(issues: list[dict[str, Any]], as_json: bool) -> None:
    if as_json:
        emit(issues, True)
        return
    print(render_table(["ID", "ESTADO", "ASIGNADO", "PRIO", "TÍTULO"], issue_rows(issues)))
    print(STYLE.dim(f"\n{len(issues)} issue(s)"))


def cmd_issues(client: Client, args: argparse.Namespace) -> None:
    team_key: str | None = None
    if args.team or not args.all_teams:
        team_key = client.team(args.team)["key"]

    assignee_id = None
    if args.assignee:
        assignee_id = client.resolve_user(args.assignee)["id"]

    state_id = (
        client.resolve_state(team_key or client.default_team(), args.state)["id"]
        if args.state
        else None
    )
    issues = client.issues(
        team=team_key,
        state_id=state_id,
        assignee_id=assignee_id,
        project=args.project,
        text=args.search,
        limit=args.limit,
    )
    _print_issues(issues, args.json)


def cmd_mine(client: Client, args: argparse.Namespace) -> None:
    viewer = client.viewer()
    team_key = client.team(args.team)["key"]
    state_id = (
        client.resolve_state(team_key, args.state)["id"] if args.state else None
    )
    issues = client.issues(
        team=team_key,
        state_id=state_id,
        assignee_id=viewer["id"],
        limit=args.limit,
    )
    _print_issues(issues, args.json)


def cmd_person(client: Client, args: argparse.Namespace) -> None:
    user = client.resolve_user(args.person)
    team_key = client.team(args.team)["key"]
    state_id = (
        client.resolve_state(team_key, args.state)["id"] if args.state else None
    )
    issues = client.issues(
        team=team_key,
        state_id=state_id,
        assignee_id=user["id"],
        limit=args.limit,
    )
    if args.json:
        emit({"user": user, "issues": issues}, True)
        return
    print(f"{STYLE.bold(user['name'])} <{user['email']}>\n")
    _print_issues(issues, False)


def cmd_show(client: Client, args: argparse.Namespace) -> None:
    issue = client.issue(args.identifier, with_comments=True)
    if args.json:
        emit(issue, True)
        return

    print(STYLE.bold(f"{issue['identifier']}  {issue['title']}"))
    print(f"  estado      {(issue.get('state') or {}).get('name', '—')}")
    print(f"  asignado    {(issue.get('assignee') or {}).get('name', '—')}")
    print(f"  proyecto    {(issue.get('project') or {}).get('name', '—')}")
    print(f"  prioridad   {PRIORITY_LABELS.get(issue.get('priority', 0), '—')}")
    labels = [label["name"] for label in issue["labels"]["nodes"]]
    if labels:
        print(f"  etiquetas   {', '.join(labels)}")
    print(f"  actualizado {issue.get('updatedAt', '')[:19].replace('T', ' ')}")
    print(f"  url         {STYLE.cyan(issue.get('url', ''))}")

    description = (issue.get("description") or "").strip()
    if description:
        print("\n  descripción")
        for line in description.splitlines():
            print(f"    {line}")

    comments = (issue.get("comments") or {}).get("nodes", [])
    print(f"\n  comentarios ({len(comments)})")
    for comment in comments:
        author = (comment.get("user") or {}).get("name", "?")
        when = comment.get("createdAt", "")[:16].replace("T", " ")
        print(f"    {STYLE.dim(when)} {STYLE.bold(author)}")
        for line in (comment.get("body") or "").splitlines():
            print(f"      {line}")


def cmd_search(client: Client, args: argparse.Namespace) -> None:
    issues = client.issues(text=args.text, limit=args.limit)
    if not issues:
        warn(f"nada coincide con {args.text!r} en títulos ni descripciones")
    _print_issues(issues, args.json)


def _current_issue(client: Client, identifier: str) -> dict[str, Any]:
    return client.issue(identifier)


def _apply_state(client: Client, args: argparse.Namespace, wanted: str) -> None:
    issue = _current_issue(client, args.identifier)
    team_key = args.team or issue["identifier"].split("-")[0]
    state = client.resolve_state(team_key, wanted)

    if args.dry_run:
        print(
            f"[dry-run] issueUpdate({issue['identifier']} → stateId={state['name']} "
            f"[{state['id']}])"
        )
        return

    updated = client.update_issue(issue["id"], {"stateId": state["id"]})
    if args.json:
        emit(updated, True)
        return
    print(
        f"{STYLE.green('✓')} {updated['identifier']} → "
        f"{updated['state']['name']} ({state['type']})"
    )


def cmd_move(client: Client, args: argparse.Namespace) -> None:
    _apply_state(client, args, args.state)


def cmd_assign(client: Client, args: argparse.Namespace) -> None:
    issue = _current_issue(client, args.identifier)
    if args.person:
        user = client.resolve_user(args.person)
        changes = {"assigneeId": user["id"]}
        label = user["name"]
    else:
        changes = {"assigneeId": None}
        label = "(sin asignar)"

    if args.dry_run:
        print(f"[dry-run] issueUpdate({issue['identifier']} → assigneeId={label})")
        return

    updated = client.update_issue(issue["id"], changes)
    if args.json:
        emit(updated, True)
        return
    assignee = (updated.get("assignee") or {}).get("name", "(sin asignar)")
    print(f"{STYLE.green('✓')} {updated['identifier']} asignada a {assignee}")


def cmd_comment(client: Client, args: argparse.Namespace) -> None:
    if args.message == "-":
        body = sys.stdin.read().strip()
    elif args.message:
        body = args.message
    else:
        die("falta el texto del comentario (usa - para leer de stdin)")
        return
    if not body:
        die("el comentario está vacío")
        return

    issue = _current_issue(client, args.identifier)
    if args.dry_run:
        print(f"[dry-run] commentCreate({issue['identifier']}, {len(body)} chars)")
        return

    comment = client.add_comment(issue["id"], body)
    if args.json:
        emit(comment, True)
        return
    print(f"{STYLE.green('✓')} comentario agregado a {issue['identifier']}")
    if comment.get("url"):
        print(f"  {STYLE.cyan(comment['url'])}")


def cmd_create(client: Client, args: argparse.Namespace) -> None:
    team = client.team(args.team)
    payload: dict[str, Any] = {"teamId": team["id"], "title": args.title}
    if args.description:
        payload["description"] = args.description
    if args.assignee:
        payload["assigneeId"] = client.resolve_user(args.assignee)["id"]
    if args.priority is not None:
        payload["priority"] = args.priority
    if args.state:
        payload["stateId"] = client.resolve_state(team["key"], args.state)["id"]
    if args.project:
        payload["projectId"] = client.project_id(args.project)

    if args.dry_run:
        print("[dry-run] issueCreate")
        print(json.dumps(payload, indent=2, ensure_ascii=False))
        return

    issue = client.create_issue(payload)
    if args.json:
        emit(issue, True)
        return
    print(f"{STYLE.green('✓')} creada {issue['identifier']}: {issue['title']}")
    print(f"  {STYLE.cyan(issue.get('url', ''))}")


def cmd_open(client: Client, args: argparse.Namespace) -> None:
    issue = client.issue(args.identifier)
    url = issue["url"]
    if args.print_only:
        print(url)
        return
    opener = shutil.which("open") or shutil.which("xdg-open")
    if not opener:
        print(url)
        return
    subprocess.run([opener, url], check=False)


def cmd_raw(client: Client, args: argparse.Namespace) -> None:
    query = args.query
    if query == "-":
        query = sys.stdin.read()
    elif query.startswith("@"):
        query = Path(query[1:]).read_text(encoding="utf-8")
    variables = json.loads(args.vars) if args.vars else None
    emit(client.gql(query, variables), True)


# --------------------------------------------------------------------------
# Parser
# --------------------------------------------------------------------------


def build_parser() -> argparse.ArgumentParser:
    # Estas opciones valen tanto antes como después del subcomando.
    # default=SUPPRESS evita que el subparser pise el valor ya leído por el
    # parser principal cuando el flag se escribió antes del comando.
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument(
        "--json", action="store_true", default=argparse.SUPPRESS, help="salida JSON cruda"
    )
    common.add_argument(
        "--no-color",
        action="store_true",
        default=argparse.SUPPRESS,
        help="desactiva colores ANSI",
    )
    common.add_argument(
        "--team",
        default=argparse.SUPPRESS,
        help="clave del equipo (por defecto el único del workspace)",
    )
    common.add_argument(
        "--dry-run",
        action="store_true",
        default=argparse.SUPPRESS,
        help="muestra la mutación que enviaría, sin ejecutarla",
    )

    parser = argparse.ArgumentParser(
        prog="linear",
        description="CLI de Linear para SOLOServis (sin dependencias externas).",
        epilog="La API key se toma de LINEAR_API_KEY o del .env del repositorio.",
        parents=[common],
    )

    sub = parser.add_subparsers(dest="command", required=True)

    def add(name: str, **kwargs: Any) -> argparse.ArgumentParser:
        return sub.add_parser(name, parents=[common], **kwargs)

    add("whoami", help="usuario dueño de la API key").set_defaults(func=cmd_whoami)

    add("teams", help="equipos del workspace").set_defaults(func=cmd_teams)

    users = add("users", help="personas del workspace")
    users.add_argument("--all", action="store_true", help="incluye inactivos")
    users.set_defaults(func=cmd_users)

    states = add("states", help="estados del flujo de trabajo")
    states.set_defaults(func=cmd_states)

    add("projects", help="proyectos y su avance").set_defaults(
        func=cmd_projects
    )

    stats = add("stats", help="resumen por estado y por persona")
    stats.add_argument("--limit", type=int, default=1000, help="issues a considerar")
    stats.set_defaults(func=cmd_stats)

    issues = add("issues", aliases=["ls"], help="lista issues")
    issues.add_argument("--state", help="filtrar por nombre de estado")
    issues.add_argument("--assignee", help="filtrar por persona (nombre o email)")
    issues.add_argument("--project", help="filtrar por proyecto")
    issues.add_argument("--search", help="texto en título o descripción")
    issues.add_argument("--limit", type=int, default=25, help="máximo de resultados")
    issues.add_argument(
        "--all-teams", action="store_true", help="no filtrar por equipo"
    )
    issues.set_defaults(func=cmd_issues)

    mine = add("mine", aliases=["me"], help="mis issues asignadas")
    mine.add_argument("--state", help="filtrar por estado")
    mine.add_argument("--limit", type=int, default=25)
    mine.set_defaults(func=cmd_mine)

    person = add("person", help="issues asignadas a alguien")
    person.add_argument("person", help="nombre o email")
    person.add_argument("--state", help="filtrar por estado")
    person.add_argument("--limit", type=int, default=25)
    person.set_defaults(func=cmd_person)

    show = add("show", aliases=["get"], help="detalle de una issue")
    show.add_argument("identifier", help="por ejemplo TAL-233")
    show.set_defaults(func=cmd_show)

    search = add("search", help="busca en títulos y descripciones")
    search.add_argument("text")
    search.add_argument("--limit", type=int, default=25)
    search.set_defaults(func=cmd_search)

    for name, aliases, text in (
        ("start", ["doing"], "mueve a In Progress"),
        ("done", ["complete"], "mueve a Done"),
        ("todo", [], "mueve a Todo"),
        ("review", [], "mueve a In Review"),
        ("backlog", [], "mueve a Backlog"),
    ):
        sub_parser = add(name, aliases=aliases, help=text)
        sub_parser.add_argument("identifier", help="por ejemplo TAL-233")
        sub_parser.set_defaults(
            func=lambda client, args, target=name: _apply_state(client, args, target)
        )

    move = add("move", aliases=["mv"], help="mueve a un estado cualquiera")
    move.add_argument("identifier")
    move.add_argument("state", help="nombre o alias del estado")
    move.set_defaults(func=cmd_move)

    assign = add("assign", help="asigna o quita el responsable")
    assign.add_argument("identifier")
    assign.add_argument("person", nargs="?", help="nombre o email; omítelo para desasignar")
    assign.set_defaults(func=cmd_assign)

    comment = add("comment", help="agrega un comentario")
    comment.add_argument("identifier")
    comment.add_argument("message", nargs="?", help="texto; '-' lee de stdin")
    comment.set_defaults(func=cmd_comment)

    create = add("create", aliases=["new"], help="crea una issue")
    create.add_argument("--title", required=True)
    create.add_argument("--description", "-d")
    create.add_argument("--assignee", "-a", help="nombre o email")
    create.add_argument("--state", help="estado inicial")
    create.add_argument("--project", help="proyecto (por nombre)")
    create.add_argument(
        "--priority",
        "-p",
        type=int,
        choices=[0, 1, 2, 3, 4],
        help="0 sin prioridad, 1 urgent … 4 low",
    )
    create.set_defaults(func=cmd_create)

    open_parser = add("open", help="abre la issue en el navegador")
    open_parser.add_argument("identifier")
    open_parser.add_argument("--print-only", action="store_true")
    open_parser.set_defaults(func=cmd_open)

    raw = add("raw", help="consulta GraphQL cruda")
    raw.add_argument("query", help="consulta, @archivo.graphql o '-' para stdin")
    raw.add_argument("--vars", help="variables JSON")
    raw.set_defaults(func=cmd_raw)

    return parser


def main(argv: list[str] | None = None) -> int:
    global STYLE
    parser = build_parser()
    args = parser.parse_args(argv)
    # Los flags comunes usan default=SUPPRESS para no pisarse entre el parser
    # principal y el del subcomando; aquí se rellenan los ausentes.
    for name, default in (
        ("json", False),
        ("no_color", False),
        ("team", None),
        ("dry_run", False),
    ):
        if not hasattr(args, name):
            setattr(args, name, default)
    if args.no_color:
        STYLE = Style(False)

    try:
        client = Client(find_api_key())
        args.func(client, args)
    except LinearError as exc:
        die(str(exc))
    except KeyboardInterrupt:
        print("\ninterrumpido", file=sys.stderr)
        return 130
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
