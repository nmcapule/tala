#!/usr/bin/env python3
import argparse
import json
import mimetypes
import os
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


DEFAULT_URL = "http://127.0.0.1:8081"


def project_root() -> Path:
    current = Path.cwd().resolve()
    for path in [current, *current.parents]:
        if (path / ".tala" / "tala.db").exists() or (path / "tala.db").exists() or (path / "Makefile").exists() or (path / ".git").exists():
            return path
    return current


def request_json(method: str, url: str, payload=None, username=None):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if username:
        headers["X-Tala-Username"] = username
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            text = res.read().decode("utf-8")
            return json.loads(text) if text else None
    except urllib.error.HTTPError as err:
        text = err.read().decode("utf-8")
        raise SystemExit(f"{method} {url} failed with HTTP {err.code}: {text}")
    except urllib.error.URLError as err:
        raise SystemExit(f"{method} {url} failed: {err.reason}")


def request_multipart_image(url: str, path: Path, username=None):
    if not path.exists():
        raise SystemExit(f"image path not found: {path}")
    if not path.is_file():
        raise SystemExit(f"image path is not a file: {path}")
    boundary = "----tala-upload-" + os.urandom(12).hex()
    content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
    head = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="image"; filename="{path.name}"\r\n'
        f"Content-Type: {content_type}\r\n\r\n"
    ).encode("utf-8")
    tail = f"\r\n--{boundary}--\r\n".encode("utf-8")
    data = head + path.read_bytes() + tail
    headers = {
        "Accept": "application/json",
        "Content-Type": f"multipart/form-data; boundary={boundary}",
    }
    if username:
        headers["X-Tala-Username"] = username
    req = urllib.request.Request(url, data=data, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            text = res.read().decode("utf-8")
            return json.loads(text) if text else None
    except urllib.error.HTTPError as err:
        text = err.read().decode("utf-8")
        raise SystemExit(f"POST {url} failed with HTTP {err.code}: {text}")
    except urllib.error.URLError as err:
        raise SystemExit(f"POST {url} failed: {err.reason}")


def mcp(url: str, method: str, params=None):
    payload = {"jsonrpc": "2.0", "id": 1, "method": method}
    if params is not None:
        payload["params"] = params
    req = urllib.request.Request(
        f"{url.rstrip('/')}/mcp",
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            body = json.loads(res.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        text = err.read().decode("utf-8")
        raise SystemExit(f"MCP request failed with HTTP {err.code}: {text}")
    if body.get("error"):
        raise SystemExit(json.dumps(body["error"], indent=2))
    return body.get("result")


def print_json(value):
    print(json.dumps(value, indent=2, sort_keys=True))


def cmd_health(args):
    print_json(request_json("GET", f"{args.url.rstrip('/')}/healthz"))


def cmd_serve(args):
    root = project_root()
    db = Path(args.db).expanduser()
    if not db.is_absolute():
        db = root / db
    if (root / "Makefile").exists():
        cmd = ["make", "own-db", f"OWN_DB_ADDR={urllib.parse.urlparse(args.url).netloc}", f"OWN_DB={db}"]
    else:
        cmd = ["go", "run", "./cmd/tala", "-addr", urllib.parse.urlparse(args.url).netloc, "-db", str(db)]
    os.execvp(cmd[0], cmd)


def cmd_search(args):
    query = {}
    for key in ["q", "status", "priority", "assignee", "tag", "parent_id", "blocked_by"]:
        value = getattr(args, key)
        if value:
            query[key] = value
    path = "/api/issues"
    if query:
        path += "?" + urllib.parse.urlencode(query)
    print_json(request_json("GET", f"{args.url.rstrip('/')}{path}"))


def cmd_planning(args):
    result = mcp(args.url, "resources/read", {"uri": "tala://planning"})
    contents = result.get("contents", [])
    if contents and isinstance(contents[0].get("text"), str):
        print(contents[0]["text"])
    else:
        print_json(result)


def cmd_create(args):
    body = {
        "title": args.title,
        "description_markdown": args.description or "",
        "priority": args.priority,
        "assignee": args.assignee,
        "parent_issue_id": args.parent_id,
        "tag_names": args.tag or [],
    }
    print_json(request_json("POST", f"{args.url.rstrip('/')}/api/issues", body, args.username))


def cmd_comment(args):
    if args.body_file:
        body = Path(args.body_file).read_text()
    else:
        body = args.body
    print_json(request_json(
        "POST",
        f"{args.url.rstrip('/')}/api/issues/{args.issue_id}/comments",
        {"body_markdown": body},
        args.username,
    ))


def cmd_upload_image(args):
    uploaded = request_multipart_image(
        f"{args.url.rstrip('/')}/api/uploads/images",
        Path(args.path).expanduser(),
        args.username,
    )
    alt_text = (args.alt_text or Path(args.path).name).strip() or "uploaded image"
    uploaded["markdown"] = f"![{alt_text}]({uploaded['url']})"
    print_json(uploaded)


def cmd_set_status(args):
    print_json(request_json(
        "PATCH",
        f"{args.url.rstrip('/')}/api/issues/{args.issue_id}",
        {"status": args.status},
        args.username,
    ))


def cmd_set_parent(args):
    print_json(request_json(
        "PUT",
        f"{args.url.rstrip('/')}/api/issues/{args.issue_id}/parent",
        {"parent_issue_id": args.parent_id},
        args.username,
    ))


def cmd_add_blocker(args):
    print_json(request_json(
        "POST",
        f"{args.url.rstrip('/')}/api/issues/{args.issue_id}/blockers",
        {"blocker_issue_id": args.blocker_issue_id},
        args.username,
    ))


def main():
    parser = argparse.ArgumentParser(description="Interact with a project-local Tala server.")
    parser.add_argument("--url", default=os.environ.get("TALA_URL", DEFAULT_URL))
    parser.add_argument("--username", default=os.environ.get("TALA_USERNAME", "agent"))
    parser.add_argument("--db", default=os.environ.get("TALA_DB", ".tala/tala.db"))
    sub = parser.add_subparsers(required=True)

    health = sub.add_parser("health")
    health.set_defaults(func=cmd_health)

    serve = sub.add_parser("serve")
    serve.set_defaults(func=cmd_serve)

    search = sub.add_parser("search")
    for name in ["q", "status", "priority", "assignee", "tag", "parent_id", "blocked_by"]:
        search.add_argument(f"--{name}")
    search.set_defaults(func=cmd_search)

    planning = sub.add_parser("planning")
    planning.set_defaults(func=cmd_planning)

    create = sub.add_parser("create")
    create.add_argument("--title", required=True)
    create.add_argument("--description")
    create.add_argument("--priority", default="P2", choices=["P0", "P1", "P2", "P3", "P4"])
    create.add_argument("--assignee")
    create.add_argument("--parent-id")
    create.add_argument("--tag", action="append")
    create.set_defaults(func=cmd_create)

    comment = sub.add_parser("comment")
    comment.add_argument("--issue-id", required=True)
    group = comment.add_mutually_exclusive_group(required=True)
    group.add_argument("--body")
    group.add_argument("--body-file")
    comment.set_defaults(func=cmd_comment)

    upload_image = sub.add_parser("upload-image")
    upload_image.add_argument("--path", required=True)
    upload_image.add_argument("--alt-text")
    upload_image.set_defaults(func=cmd_upload_image)

    set_status = sub.add_parser("set-status")
    set_status.add_argument("--issue-id", required=True)
    set_status.add_argument("--status", required=True, choices=["new", "in_progress", "completed", "canceled"])
    set_status.set_defaults(func=cmd_set_status)

    set_parent = sub.add_parser("set-parent")
    set_parent.add_argument("--issue-id", required=True)
    set_parent.add_argument("--parent-id")
    set_parent.set_defaults(func=cmd_set_parent)

    add_blocker = sub.add_parser("add-blocker")
    add_blocker.add_argument("--issue-id", required=True)
    add_blocker.add_argument("--blocker-issue-id", required=True)
    add_blocker.set_defaults(func=cmd_add_blocker)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
