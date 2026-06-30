#!/usr/bin/env python3
"""Check OBTC parameter consistency across code and selected docs.

The checker is intentionally narrow and dependency-free.  It reads the manifest
for the values that are already confirmed by code/review docs, extracts those
values from source files, and checks explicitly listed documentation references.
It does not decide which side is right when values disagree.
"""

from __future__ import annotations

import argparse
import ast
import fnmatch
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable


STATUS_OK = "exact match"
STATUS_MISSING = "missing"
STATUS_MISMATCH = "mismatch"
STATUS_AMBIGUOUS = "ambiguous"
STATUS_SKIPPED = "skipped"
STATUS_ERROR = "error"
STATUS_WARNING = "warning"

FAIL_STATUSES = {STATUS_MISSING, STATUS_MISMATCH, STATUS_AMBIGUOUS, STATUS_ERROR}


@dataclass(frozen=True)
class CheckResult:
    parameter_id: str
    parameter_name: str
    expected: str
    category: str
    role: str
    repo: str
    path: str
    required: bool
    status: str
    observed: str
    detail: str


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(
        description="Check OBTC mainnet-candidate parameter consistency."
    )
    parser.add_argument(
        "--repo-root",
        default=str(repo_root),
        help="Path to the obtcd checkout. Defaults to this script's repository.",
    )
    parser.add_argument(
        "--workspace-root",
        default=None,
        help="Workspace root containing sibling repos. Defaults to repo-root parent.",
    )
    parser.add_argument(
        "--manifest",
        default=None,
        help="Parameter manifest path. Defaults to config/obtc-mainnet-candidate-parameters.json.",
    )
    parser.add_argument(
        "--format",
        choices=("markdown", "json"),
        default="markdown",
        help="Output format.",
    )
    parser.add_argument(
        "--strict-optional",
        action="store_true",
        help="Return non-zero when optional cross-repo references mismatch or are missing.",
    )
    return parser.parse_args()


def strip_go_line_comment(line: str) -> str:
    in_string = False
    escaped = False
    quote = ""
    for i, ch in enumerate(line):
        if in_string:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == quote:
                in_string = False
            continue
        if ch in ('"', "`"):
            in_string = True
            quote = ch
            continue
        if ch == "/" and i + 1 < len(line) and line[i + 1] == "/":
            return line[:i]
    return line


def parse_go_consts(text: str) -> dict[str, str]:
    raw: dict[str, str] = {}
    in_block = False
    for line in text.splitlines():
        clean = strip_go_line_comment(line).strip()
        if not clean:
            continue
        if clean.startswith("const ("):
            in_block = True
            continue
        if in_block and clean == ")":
            in_block = False
            continue
        if clean.startswith("const "):
            clean = clean[len("const ") :].strip()
        elif not in_block:
            continue

        match = re.match(
            r"^([A-Za-z_][A-Za-z0-9_]*)(?:\s+[A-Za-z_][A-Za-z0-9_\.\[\]]*)?\s*=\s*(.+)$",
            clean,
        )
        if not match:
            continue
        name, expr = match.groups()
        raw[name] = expr.rstrip(",").strip()
    return raw


class ConstEvaluator:
    def __init__(self, raw_consts: dict[str, str]) -> None:
        self.raw_consts = raw_consts
        self.resolved: dict[str, str] = {}

    def resolve(self, name: str) -> str:
        if name in self.resolved:
            return self.resolved[name]
        if name not in self.raw_consts:
            raise KeyError(f"constant {name} not found")
        value = self.eval_expr(self.raw_consts[name])
        self.resolved[name] = value
        return value

    def eval_expr(self, expr: str) -> str:
        expr = expr.strip().rstrip(",")
        if expr.startswith('"') and expr.endswith('"'):
            return expr.strip('"')
        tree = ast.parse(expr.replace("_", ""), mode="eval")
        value = self._eval_ast(tree.body)
        return str(value)

    def _eval_ast(self, node: ast.AST) -> int:
        if isinstance(node, ast.Constant) and isinstance(node.value, int):
            return int(node.value)
        if isinstance(node, ast.Name):
            return int(self.resolve(node.id))
        if isinstance(node, ast.BinOp):
            left = self._eval_ast(node.left)
            right = self._eval_ast(node.right)
            if isinstance(node.op, ast.Add):
                return left + right
            if isinstance(node.op, ast.Sub):
                return left - right
            if isinstance(node.op, ast.Mult):
                return left * right
            if isinstance(node.op, ast.FloorDiv):
                return left // right
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.USub):
            return -self._eval_ast(node.operand)
        raise ValueError(f"unsupported Go constant expression: {ast.dump(node)}")


class Extractor:
    def __init__(self, repo_root: Path, workspace_root: Path) -> None:
        self.repo_root = repo_root
        self.workspace_root = workspace_root
        self.text_cache: dict[Path, str] = {}
        self.const_cache: dict[Path, ConstEvaluator] = {}

    def repo_path(self, repo: str) -> Path:
        if repo == "obtcd":
            return self.repo_root
        return self.workspace_root / repo

    def read_text(self, repo: str, rel_path: str) -> str:
        path = self.repo_path(repo) / rel_path
        if path not in self.text_cache:
            self.text_cache[path] = path.read_text(encoding="utf-8")
        return self.text_cache[path]

    def path_exists(self, repo: str, rel_path: str) -> bool:
        return (self.repo_path(repo) / rel_path).is_file()

    def consts(self, repo: str, rel_path: str) -> ConstEvaluator:
        path = self.repo_path(repo) / rel_path
        if path not in self.const_cache:
            self.const_cache[path] = ConstEvaluator(parse_go_consts(self.read_text(repo, rel_path)))
        return self.const_cache[path]

    def code_value(self, source: dict[str, Any]) -> str:
        kind = source["extractor"]
        repo = source.get("repo", "obtcd")
        rel_path = source["path"]
        if kind == "go_const":
            return self.consts(repo, rel_path).resolve(source["name"])
        if kind == "go_composite_field":
            body = self.go_composite_body(repo, rel_path, source["object"])
            return self.go_field_value(body, source["field"], repo, rel_path)
        if kind == "go_expiry_field":
            body = self.go_expiry_body(repo, rel_path, source["case"])
            return self.go_field_value(body, source["field"], repo, rel_path)
        if kind == "go_expiry_ratio":
            body = self.go_expiry_body(repo, rel_path, source["case"])
            numerator = int(self.go_field_value(body, source["numerator"], repo, rel_path))
            denominator = int(self.go_field_value(body, source["denominator"], repo, rel_path))
            if source.get("mode") == "refund":
                numerator = denominator - numerator
            return f"{numerator}/{denominator}"
        if kind == "regex":
            text = self.read_text(repo, rel_path)
            return regex_values(text, source["regex"], source.get("flags", ""))[0]
        raise ValueError(f"unknown extractor: {kind}")

    def go_composite_body(self, repo: str, rel_path: str, object_name: str) -> str:
        text = self.read_text(repo, rel_path)
        pattern = re.compile(
            r"\bvar\s+"
            + re.escape(object_name)
            + r"\s*=\s*(?:[A-Za-z_][A-Za-z0-9_\.]*\s*)?\{",
            re.MULTILINE,
        )
        match = pattern.search(text)
        if not match:
            raise ValueError(f"composite {object_name} not found")
        open_brace = text.rfind("{", 0, match.end())
        return balanced_brace_body(text, open_brace)

    def go_expiry_body(self, repo: str, rel_path: str, case_name: str) -> str:
        text = self.read_text(repo, rel_path)
        func_match = re.search(r"\bfunc\s+GetExpiryParams\s*\(", text)
        if not func_match:
            raise ValueError("GetExpiryParams function not found")
        func_open = text.find("{", func_match.end())
        func_body = balanced_brace_body(text, func_open)
        case_match = re.search(r"\bcase\s+" + re.escape(case_name) + r"\s*:", func_body)
        if not case_match:
            raise ValueError(f"expiry case {case_name} not found")
        return_match = re.search(r"return\s+&?ExpiryParams\s*\{", func_body[case_match.end() :])
        if not return_match:
            raise ValueError(f"ExpiryParams return for {case_name} not found")
        open_brace = case_match.end() + return_match.end() - 1
        return balanced_brace_body(func_body, open_brace)

    def go_field_value(self, body: str, field: str, repo: str, rel_path: str) -> str:
        field_re = re.compile(
            r"(?m)^\s*" + re.escape(field) + r"\s*:\s*(?P<value>.+?)(?:,\s*)?$"
        )
        match = field_re.search(body)
        if not match:
            raise ValueError(f"field {field} not found")
        raw = match.group("value").strip()
        if raw.startswith('"') and raw.endswith('"'):
            return raw.strip('"')
        if raw.startswith("[4]byte"):
            parts = re.findall(r"0x[0-9A-Fa-f]{2}", raw)
            return "".join(part[2:].upper() for part in parts)
        if re.fullmatch(r"0x[0-9A-Fa-f]+", raw):
            return "0x" + raw[2:].upper()
        if re.fullmatch(r"[0-9][0-9_,]*", raw):
            return str(int(raw.replace("_", "").replace(",", "")))
        if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*(?:\s*[+-]\s*[0-9][0-9_]*)?", raw):
            consts = self.consts(repo, rel_path)
            return consts.eval_expr(raw)
        raise ValueError(f"unsupported field value for {field}: {raw}")


def balanced_brace_body(text: str, open_brace: int) -> str:
    depth = 0
    in_string = False
    quote = ""
    escaped = False
    for i in range(open_brace, len(text)):
        ch = text[i]
        if in_string:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == quote:
                in_string = False
            continue
        if ch in ('"', "`"):
            in_string = True
            quote = ch
            continue
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                return text[open_brace + 1 : i]
    raise ValueError("unbalanced Go composite literal")


def regex_values(text: str, pattern: str, flags_text: str = "") -> list[str]:
    flags = re.MULTILINE
    if "s" in flags_text:
        flags |= re.DOTALL
    regex = re.compile(pattern, flags)
    values: list[str] = []
    for match in regex.finditer(text):
        if "value" in match.groupdict():
            values.append(match.group("value"))
        elif match.groups():
            values.append(match.group(1))
        else:
            values.append(match.group(0))
    return values


def normalize(value: str, value_type: str) -> str:
    raw = str(value).strip().strip("`").strip()
    if value_type in {"int", "port"}:
        digits = re.sub(r"[\s,_]", "", raw)
        if re.fullmatch(r"0x[0-9A-Fa-f]+", digits):
            return str(int(digits, 16))
        match = re.search(r"-?\d+", digits)
        if not match:
            return raw
        return str(int(match.group(0)))
    if value_type == "hex":
        compact = raw.replace("_", "").replace(",", "").strip()
        if compact.lower().startswith("0x"):
            return "0x" + compact[2:].upper()
        if re.fullmatch(r"\d+", compact):
            return f"0x{int(compact):X}"
        return "0x" + compact.upper()
    if value_type == "hd_key":
        compact = re.sub(r"[^0-9A-Fa-f]", "", raw)
        return compact.upper()
    if value_type == "ratio":
        compact = raw.lower().replace(" ", "")
        compact = compact.replace("percent", "%")
        if compact.endswith("%"):
            pct = compact[:-1]
            if pct in {"30", "70"}:
                return f"{pct}/100"
        if compact in {"0.30", ".30"}:
            return "30/100"
        if compact in {"0.70", ".70"}:
            return "70/100"
        return compact
    return raw.strip('"').strip("'")


def acceptable_values(parameter: dict[str, Any]) -> set[str]:
    value_type = parameter.get("value_type", "string")
    values = [parameter["expected"]] + parameter.get("aliases", [])
    return {normalize(str(value), value_type) for value in values}


def check_code_source(
    extractor: Extractor, parameter: dict[str, Any], source: dict[str, Any]
) -> CheckResult:
    repo = source.get("repo", "obtcd")
    rel_path = source["path"]
    required = source.get("required", True)
    expected = str(parameter["expected"])
    if not extractor.path_exists(repo, rel_path):
        return CheckResult(
            parameter["id"],
            parameter["name"],
            expected,
            parameter["category"],
            "code",
            repo,
            rel_path,
            required,
            STATUS_MISSING if required else STATUS_SKIPPED,
            "",
            "source file not found",
        )
    try:
        observed = extractor.code_value(source)
    except Exception as exc:  # noqa: BLE001 - report extractor failures as data.
        return CheckResult(
            parameter["id"],
            parameter["name"],
            expected,
            parameter["category"],
            "code",
            repo,
            rel_path,
            required,
            STATUS_ERROR,
            "",
            str(exc),
        )
    normalized = normalize(observed, parameter.get("value_type", "string"))
    status = STATUS_OK if normalized in acceptable_values(parameter) else STATUS_MISMATCH
    return CheckResult(
        parameter["id"],
        parameter["name"],
        expected,
        parameter["category"],
        "code",
        repo,
        rel_path,
        required,
        status,
        observed,
        source.get("label", source.get("extractor", "")),
    )


def check_doc_ref(
    extractor: Extractor, parameter: dict[str, Any], ref: dict[str, Any]
) -> CheckResult:
    repo = ref.get("repo", "obtcd")
    rel_path = ref["path"]
    required = ref.get("required", False)
    expected = str(parameter["expected"])
    if not extractor.path_exists(repo, rel_path):
        return CheckResult(
            parameter["id"],
            parameter["name"],
            expected,
            parameter["category"],
            "docs",
            repo,
            rel_path,
            required,
            STATUS_MISSING if required else STATUS_SKIPPED,
            "",
            "document not found",
        )
    text = extractor.read_text(repo, rel_path)
    values: list[str]
    if "regex" in ref:
        values = regex_values(text, ref["regex"], ref.get("flags", ""))
    else:
        values = [literal for literal in ref.get("literals", []) if literal in text]
    if not values:
        return CheckResult(
            parameter["id"],
            parameter["name"],
            expected,
            parameter["category"],
            "docs",
            repo,
            rel_path,
            required,
            STATUS_MISSING,
            "",
            ref.get("label", "reference not found"),
        )
    normalized_values = {
        normalize(value, parameter.get("value_type", "string")) for value in values
    }
    accepted = acceptable_values(parameter)
    if normalized_values <= accepted:
        status = STATUS_OK
    elif normalized_values & accepted:
        status = STATUS_AMBIGUOUS
    else:
        status = STATUS_MISMATCH
    return CheckResult(
        parameter["id"],
        parameter["name"],
        expected,
        parameter["category"],
        "docs",
        repo,
        rel_path,
        required,
        status,
        ", ".join(values[:4]) + (" ..." if len(values) > 4 else ""),
        ref.get("label", "document reference"),
    )


def expand_glob_refs(
    extractor: Extractor, refs: Iterable[dict[str, Any]]
) -> list[dict[str, Any]]:
    expanded: list[dict[str, Any]] = []
    for ref in refs:
        path = ref["path"]
        if not any(char in path for char in "*?["):
            expanded.append(ref)
            continue
        repo = ref.get("repo", "obtcd")
        base = extractor.repo_path(repo)
        if not base.is_dir():
            expanded.append(ref)
            continue
        matches = [
            p.relative_to(base).as_posix()
            for p in base.rglob("*")
            if p.is_file() and fnmatch.fnmatch(p.relative_to(base).as_posix(), path)
        ]
        if not matches:
            expanded.append(ref)
            continue
        for match in sorted(matches):
            copy = dict(ref)
            copy["path"] = match
            expanded.append(copy)
    return expanded


def run_checks(
    manifest: dict[str, Any], extractor: Extractor
) -> list[CheckResult]:
    results: list[CheckResult] = []
    for parameter in manifest["parameters"]:
        for source in parameter.get("code_sources", []):
            results.append(check_code_source(extractor, parameter, source))
        for ref in expand_glob_refs(extractor, parameter.get("doc_refs", [])):
            results.append(check_doc_ref(extractor, parameter, ref))
    return results


def parameter_statuses(results: list[CheckResult]) -> dict[str, str]:
    statuses: dict[str, str] = {}
    by_param: dict[str, list[CheckResult]] = {}
    for result in results:
        by_param.setdefault(result.parameter_id, []).append(result)
    for parameter_id, rows in by_param.items():
        required_bad = [row for row in rows if row.required and row.status in FAIL_STATUSES]
        optional_bad = [
            row
            for row in rows
            if not row.required and row.status in {STATUS_MISMATCH, STATUS_AMBIGUOUS, STATUS_ERROR}
        ]
        if required_bad:
            order = [STATUS_MISMATCH, STATUS_MISSING, STATUS_AMBIGUOUS, STATUS_ERROR]
            statuses[parameter_id] = next(
                status for status in order if any(row.status == status for row in required_bad)
            )
        elif optional_bad:
            statuses[parameter_id] = STATUS_WARNING
        else:
            statuses[parameter_id] = STATUS_OK
    return statuses


def markdown_table(headers: list[str], rows: list[list[str]]) -> str:
    def esc(value: str) -> str:
        return str(value).replace("\n", " ").replace("|", "\\|")

    out = ["| " + " | ".join(headers) + " |"]
    out.append("| " + " | ".join("---" for _ in headers) + " |")
    for row in rows:
        out.append("| " + " | ".join(esc(cell) for cell in row) + " |")
    return "\n".join(out)


def render_markdown(manifest: dict[str, Any], results: list[CheckResult]) -> str:
    statuses = parameter_statuses(results)
    by_param: dict[str, list[CheckResult]] = {}
    for result in results:
        by_param.setdefault(result.parameter_id, []).append(result)

    counts: dict[str, int] = {}
    for status in statuses.values():
        counts[status] = counts.get(status, 0) + 1

    lines = [
        "# OBTC Parameter Consistency Report",
        "",
        f"Manifest: `{manifest.get('manifest_id', 'unknown')}`",
        "",
        "## Summary",
        "",
        markdown_table(
            ["Status", "Parameter count"],
            [[status, str(count)] for status, count in sorted(counts.items())],
        ),
        "",
        "## Parameters",
        "",
    ]

    parameter_rows: list[list[str]] = []
    for parameter in manifest["parameters"]:
        rows = by_param.get(parameter["id"], [])
        code_statuses = sorted({row.status for row in rows if row.role == "code"})
        doc_statuses = sorted({row.status for row in rows if row.role == "docs"})
        parameter_rows.append(
            [
                parameter["id"],
                parameter["name"],
                parameter["category"],
                str(parameter["expected"]),
                ", ".join(code_statuses) or "n/a",
                ", ".join(doc_statuses) or "n/a",
                statuses.get(parameter["id"], STATUS_MISSING),
            ]
        )
    lines.append(
        markdown_table(
            ["ID", "Parameter", "Category", "Expected", "Code", "Docs", "Status"],
            parameter_rows,
        )
    )

    findings = [
        row
        for row in results
        if row.status in FAIL_STATUSES or row.status == STATUS_WARNING
    ]
    optional_bad = [
        row
        for row in results
        if not row.required and row.status in {STATUS_MISMATCH, STATUS_AMBIGUOUS, STATUS_ERROR}
    ]
    if findings or optional_bad:
        lines.extend(["", "## Findings", ""])
        finding_rows = []
        for row in results:
            if row.status not in FAIL_STATUSES and not (
                not row.required and row.status in {STATUS_MISMATCH, STATUS_AMBIGUOUS, STATUS_ERROR}
            ):
                continue
            finding_rows.append(
                [
                    row.status if row.required else f"optional {row.status}",
                    row.parameter_id,
                    row.role,
                    f"{row.repo}/{row.path}",
                    row.expected,
                    row.observed,
                    row.detail,
                ]
            )
        lines.append(
            markdown_table(
                ["Status", "Parameter", "Role", "Path", "Expected", "Observed", "Detail"],
                finding_rows,
            )
        )
    else:
        lines.extend(["", "## Findings", "", "No required mismatches, missing references, or ambiguous references found."])

    lines.extend(["", "## Checked References", ""])
    reference_rows = [
        [
            row.parameter_id,
            row.role,
            f"{row.repo}/{row.path}",
            "yes" if row.required else "no",
            row.status,
            row.observed,
        ]
        for row in results
    ]
    lines.append(
        markdown_table(
            ["Parameter", "Role", "Path", "Required", "Status", "Observed"],
            reference_rows,
        )
    )
    lines.append("")
    return "\n".join(lines)


def render_json(manifest: dict[str, Any], results: list[CheckResult]) -> str:
    statuses = parameter_statuses(results)
    return json.dumps(
        {
            "manifest_id": manifest.get("manifest_id"),
            "parameter_statuses": statuses,
            "results": [row.__dict__ for row in results],
        },
        indent=2,
        sort_keys=True,
    )


def main() -> int:
    args = parse_args()
    repo_root = Path(args.repo_root).resolve()
    workspace_root = Path(args.workspace_root).resolve() if args.workspace_root else repo_root.parent
    manifest_path = (
        Path(args.manifest).resolve()
        if args.manifest
        else repo_root / "config" / "obtc-mainnet-candidate-parameters.json"
    )
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    extractor = Extractor(repo_root, workspace_root)
    results = run_checks(manifest, extractor)

    if args.format == "json":
        print(render_json(manifest, results))
    else:
        print(render_markdown(manifest, results))

    failing = [
        row
        for row in results
        if row.status in FAIL_STATUSES and (row.required or args.strict_optional)
    ]
    return 1 if failing else 0


if __name__ == "__main__":
    sys.exit(main())
