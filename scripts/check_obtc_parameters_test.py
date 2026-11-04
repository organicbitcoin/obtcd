"""Tests for the public-doc drift checks in ``check_obtc_parameters.py`` (#34).

Each case builds a small repository tree in a temporary directory and drives the
checker through ``run_drift_checks``, so the assertions cover the same code path
``make check-obtc-parameters`` runs. The three cases the issue names are here:

* passing — clean guidance produces no finding,
* failing — a closed milestone link, an unresolved host offered as a peer, and a
  link that leaves the OBTC repositories are each reported,
* historical-reference — the same text inside a dated report is allowed.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_obtc_parameters as checker  # noqa: E402 - path set up above.

CLOSED_MILESTONE = "https://github.com/organicbitcoin/obtcd/milestone/1"
UNRESOLVED_HOST = "seed1.mainnet.example"

POLICY = {
    "active_guidance": {"paths": ["README.md", "docs/**/*.md"], "exclude": []},
    "historical_guidance": [
        "MAINNET_CANDIDATE_*.md",
        "docs/heritage/**",
        "docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md",
    ],
    "closed_milestones": [
        {
            "number": 1,
            "title": "mainnet-candidate-2026-07",
            "state": "closed",
            "closed_at": "2026-11-02T10:37:24Z",
        }
    ],
    "peer_guidance_markers": ["--addpeer=", "fallback seed node"],
    "hosts": [
        {
            "host": UNRESOLVED_HOST,
            "role": "individual fallback seed node",
            "resolves": False,
            "observed_at": "2026-11-04",
        }
    ],
    "contributor_surfaces": {
        "paths": ["README.md", "docs/development/**/*.md"],
        "exclude": [],
        "allowed_owners": ["organicbitcoin"],
        "allowed_links": ["https://github.com/btcsuite/btcd"],
    },
}


class DriftCheckTestCase(unittest.TestCase):
    """A checker run against a throwaway checkout containing only ``obtcd``."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name)
        (self.root / "docs" / "development").mkdir(parents=True)
        (self.root / "docs" / "heritage").mkdir(parents=True)
        self.write("README.md", "# OBTC\n\nClean guidance.\n")
        self.write("docs/mainnet-join.md", "# Join\n\nUse `--addpeer=<host>:9527`.\n")

    def write(self, rel_path: str, text: str) -> None:
        path = self.root / rel_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def manifest(self, **overrides: object) -> dict:
        policy = dict(POLICY)
        policy.update(overrides)
        return {"manifest_id": "test", "parameters": [], "doc_drift": policy}

    def results(self, **overrides: object) -> list[checker.CheckResult]:
        extractor = checker.Extractor(self.root, self.root.parent)
        return checker.run_drift_checks(self.manifest(**overrides), extractor)

    def by_path(
        self, results: list[checker.CheckResult], check_id: str, path: str
    ) -> list[checker.CheckResult]:
        return [
            row
            for row in results
            if row.parameter_id == check_id and row.path == path
        ]

    # -- passing ---------------------------------------------------------

    def test_clean_guidance_has_no_findings(self) -> None:
        results = self.results()

        self.assertTrue(results)
        self.assertEqual(
            {row.status for row in results},
            {checker.STATUS_OK},
            [row for row in results if row.status != checker.STATUS_OK],
        )

    def test_unresolved_host_mentioned_without_a_peer_claim_is_fine(self) -> None:
        self.write(
            "docs/mainnet-dns-seed.md",
            f"# DNS\n\n`{UNRESOLVED_HOST}` is planned and does not resolve yet.\n",
        )

        results = self.results()

        self.assertEqual(
            self.by_path(
                results, "unresolved_host_shown_as_peer", "docs/mainnet-dns-seed.md"
            ),
            [],
        )

    def test_allowed_external_attribution_is_not_a_mismatch(self) -> None:
        self.write(
            "README.md",
            "Derived from [btcsuite/btcd](https://github.com/btcsuite/btcd).\n",
        )

        results = self.results()

        self.assertEqual(
            [
                row.status
                for row in self.by_path(results, "contributor_link_leaves_obtc", "README.md")
            ],
            [checker.STATUS_OK],
        )

    # -- failing ---------------------------------------------------------

    def test_closed_milestone_in_active_guidance_is_a_mismatch(self) -> None:
        self.write(
            "README.md",
            f"# OBTC\n\nThe public target is [{CLOSED_MILESTONE}]({CLOSED_MILESTONE}).\n",
        )

        results = self.results()

        findings = self.by_path(
            results, "closed_milestone_in_active_guidance", "README.md"
        )
        self.assertEqual([row.status for row in findings], [checker.STATUS_MISMATCH])
        self.assertIn("mainnet-candidate-2026-07", findings[0].observed)

    def test_unresolved_host_offered_as_a_peer_is_a_mismatch(self) -> None:
        self.write(
            "docs/mainnet-join.md",
            f"# Join\n\n```\n--addpeer={UNRESOLVED_HOST}:9527\n```\n",
        )

        results = self.results()

        findings = [
            row for row in results if row.parameter_id == "unresolved_host_shown_as_peer"
        ]
        self.assertEqual([row.status for row in findings], [checker.STATUS_MISMATCH])
        self.assertEqual(findings[0].path, "docs/mainnet-join.md")
        self.assertIn(UNRESOLVED_HOST, findings[0].observed)

    def test_contributor_link_outside_obtc_is_a_mismatch(self) -> None:
        self.write(
            "docs/development/code-contribution-guidelines.md",
            "See [the linter](https://github.com/golang/lint) before you send a patch.\n",
        )

        results = self.results()

        findings = self.by_path(
            results,
            "contributor_link_leaves_obtc",
            "docs/development/code-contribution-guidelines.md",
        )
        self.assertEqual([row.status for row in findings], [checker.STATUS_MISMATCH])
        self.assertIn("https://github.com/golang/lint", findings[0].observed)

    # -- historical ------------------------------------------------------

    def test_closed_milestone_in_a_dated_report_is_allowed(self) -> None:
        self.write(
            "docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md",
            f"# Report\n\nTracked by [{CLOSED_MILESTONE}]({CLOSED_MILESTONE}).\n",
        )

        results = self.results()

        self.assertEqual(
            self.by_path(
                results,
                "closed_milestone_in_active_guidance",
                "docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md",
            ),
            [],
        )

    def test_unresolved_host_in_a_dated_report_is_allowed(self) -> None:
        self.write(
            "docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md",
            f"# Report\n\n--addpeer={UNRESOLVED_HOST}:9527\n",
        )

        results = self.results()

        self.assertEqual(
            self.by_path(
                results,
                "unresolved_host_shown_as_peer",
                "docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md",
            ),
            [],
        )

    def test_heritage_folder_is_historical_not_active_guidance(self) -> None:
        self.write(
            "docs/heritage/btcd-notes.md",
            f"# Heritage\n\n[{CLOSED_MILESTONE}]({CLOSED_MILESTONE})\n",
        )

        results = self.results()

        self.assertEqual(
            self.by_path(
                results,
                "closed_milestone_in_active_guidance",
                "docs/heritage/btcd-notes.md",
            ),
            [],
        )

    # -- plumbing --------------------------------------------------------

    def test_glob_matches_top_level_and_nested_docs(self) -> None:
        pattern = checker.glob_to_regex("docs/**/*.md")

        self.assertTrue(pattern.match("docs/mainnet-join.md"))
        self.assertTrue(pattern.match("docs/reference/mining.md"))
        self.assertFalse(pattern.match("README.md"))
        self.assertFalse(pattern.match("docs/mainnet-join.txt"))

    def test_a_manifest_without_the_block_runs_no_checks(self) -> None:
        extractor = checker.Extractor(self.root, self.root.parent)

        self.assertEqual(checker.run_drift_checks({"parameters": []}, extractor), [])

    def test_json_report_carries_the_drift_rows(self) -> None:
        extractor = checker.Extractor(self.root, self.root.parent)
        manifest = self.manifest()

        report = json.loads(
            checker.render_json(manifest, checker.run_checks(manifest, extractor),
                                checker.run_drift_checks(manifest, extractor))
        )

        self.assertEqual(len(report["doc_drift_results"]), len(self.results()))
        self.assertEqual(
            report["doc_drift_results"][0]["category"], "public-doc-drift"
        )


if __name__ == "__main__":
    unittest.main()
