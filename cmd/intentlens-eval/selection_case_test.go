package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"intentlens/internal/explain"
)

func TestSelectionCaseRoutesBeforeLegacyFunctionLookup(t *testing.T) {
	casesRoot := filepath.Join("..", "..", "eval", "explain", "selection-cases")
	names, err := caseNames(casesRoot, "")
	if err != nil || len(names) != 1 || names[0] != "selection" {
		t.Fatalf("selection-only cases root should load its one fixture: names=%v err=%v", names, err)
	}
	caseDir := filepath.Join(casesRoot, names[0])
	r := runCase(caseDir, explain.Config{}, "", time.Second)
	if !strings.Contains(r.Error, "selection case recognized") || !strings.Contains(r.Error, "pending step 8b") {
		t.Fatalf("selection case did not take the explicit pending route: %+v", r)
	}
}

func TestSelectionFixtureDeclaresDefaultAndPendingFreshnessContracts(t *testing.T) {
	casePath := filepath.Join("..", "..", "eval", "explain", "selection-cases", "selection", "case.json")
	raw, err := os.ReadFile(casePath)
	if err != nil {
		t.Fatal(err)
	}
	var c Case
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode selection fixture using current case loader type: %v", err)
	}
	if c.Selection == nil || c.Function != "" || c.Selection.Mode != SelectionModeOnly || c.Selection.Freshness != SelectionFreshnessMissing {
		t.Fatalf("fixture must describe pure selection without legacy function lookup: %+v", c)
	}
	if c.Selection.FreshnessLabel != SelectionLabelMissing || c.Selection.RefreshHint != SelectionHintPrescanRequired {
		t.Fatalf("missing index must retain pure selection and request prescan explicitly: %+v", c.Selection)
	}
	if c.Selection.PersistResult || c.Selection.PrescanCache.Retained || c.Selection.PrescanCache.Authorized || !c.Selection.PrescanCache.SeparateAuthorizationRequired || len(c.Selection.AuthorizedEvidenceTypes) != 0 {
		t.Fatalf("default fixture conflates transient results with separately authorized prescan data: %+v", c.Selection)
	}
	if c.Selection.Authorization.RelatedSendAuthorized || c.Selection.Authorization.Provider != "" || c.Selection.Authorization.BaseURL != "" {
		t.Fatalf("default fixture must not inherit related-send authorization: %+v", c.Selection.Authorization)
	}
	if len(c.Selection.ExpectedEvidence) != 2 || c.Selection.ExpectedEvidence[0].Type != SelectionEvidenceCode || c.Selection.ExpectedEvidence[1].Type != SelectionEvidenceDiagnostic {
		t.Fatalf("default fixture must require only selection and intersecting diagnostic evidence: %+v", c.Selection.ExpectedEvidence)
	}
	variants := c.Selection.PendingVariants
	if len(variants) != 4 || variants[0].Mode != SelectionModeRelated || variants[0].Freshness != SelectionFreshnessUnknown || variants[1].CodeAnchorStatus != SelectionAnchorInsufficient || variants[2].Freshness != SelectionFreshnessHit || variants[3].Freshness != SelectionFreshnessStale {
		t.Fatalf("fixture must reserve related unknown and stale variants: %+v", c.Selection.PendingVariants)
	}
	if variants[0].FreshnessLabel != SelectionLabelUnverified || !variants[0].RefreshHintRequired || variants[0].RefreshHint != SelectionHintExplicitRefresh || variants[0].PersistResult || !variants[0].PrescanCache.Authorized || !variants[0].PrescanCache.SeparateAuthorizationRequired || !variants[0].Authorization.RelatedSendAuthorized || variants[0].Authorization.Provider == "" || variants[0].Authorization.BaseURL == "" || variants[0].Authorization.Version == "" {
		t.Fatalf("unknown related variant must be snapshot-labeled, transient, refreshable, and separately authorized for retention: %+v", variants[0])
	}
	if len(variants[0].AuthorizedEvidenceTypes) != 2 || variants[0].AuthorizedEvidenceTypes[0] != SelectionEvidenceRelation || variants[0].AuthorizedEvidenceTypes[1] != SelectionEvidenceDocument {
		t.Fatalf("related variant must enumerate only the approved association evidence types: %+v", variants[0].AuthorizedEvidenceTypes)
	}
	for _, ev := range variants[0].ExpectedEvidence {
		if ev.Type == SelectionEvidenceRelation || ev.Type == SelectionEvidenceDocument {
			if ev.SupportTier == "" || !ev.SnapshotTemporal || ev.Provenance.File == "" || ev.Provenance.Hash == "" || ev.Provenance.StartLine < 1 || ev.Provenance.EndLine < ev.Provenance.StartLine {
				t.Fatalf("associated evidence is missing tier or provenance: %+v", ev)
			}
		}
	}
	for _, ev := range variants[1].ExpectedEvidence {
		if ev.Type == SelectionEvidenceRelation {
			t.Fatalf("unknown snapshot with insufficient code anchor must omit code relations: %+v", ev)
		}
	}
	if len(variants[3].ExpectedEvidence) != 2 || variants[3].RefreshHint != "explicit_refresh" || !variants[3].RefreshHintRequired {
		t.Fatalf("stale contract must return only selection/diagnostics and request explicit refresh: %+v", variants[3])
	}
	for _, ev := range variants[3].ExpectedEvidence {
		if ev.Type != SelectionEvidenceCode && ev.Type != SelectionEvidenceDiagnostic {
			t.Fatalf("stale contract must not return old association evidence: %+v", ev)
		}
	}
	gotTiers := map[SelectionSupportLevel]bool{}
	for _, ev := range variants[2].ExpectedEvidence {
		if ev.SupportTier != "" {
			gotTiers[ev.SupportTier] = true
		}
	}
	if SelectionTierNameCandidate != "name_candidate" {
		t.Fatalf("D08 name candidate tier changed: %q", SelectionTierNameCandidate)
	}
	for _, tier := range []SelectionSupportLevel{SelectionTierResolvedDirect, SelectionTierDocMention, SelectionTierDocDecision} {
		if !gotTiers[tier] {
			t.Errorf("hit fixture is missing D08 tier %q", tier)
		}
	}
	forbiddenCandidateAsFact := false
	for _, rule := range variants[2].Forbid {
		if rule.Kind == SelectionForbidCandidateAsFact && rule.SupportTier == SelectionTierNameCandidate && rule.ClaimStatus == SelectionClaimSupported {
			forbiddenCandidateAsFact = true
		}
	}
	if !forbiddenCandidateAsFact {
		t.Error("D08 name candidate must be forbidden from upgrading to supported fact")
	}
	allEvidence := append([]SelectionEvidence{}, c.Selection.ExpectedEvidence...)
	for _, v := range variants {
		allEvidence = append(allEvidence, v.ExpectedEvidence...)
	}
	for _, ev := range allEvidence {
		if ev.Provenance.File == "" || ev.Provenance.Hash == "" {
			t.Fatalf("evidence is missing local provenance: %+v", ev)
		}
		path := filepath.Join("..", "..", "eval", "explain", "selection-cases", "selection", filepath.FromSlash(ev.Provenance.File))
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("provenance source %q is absent: %v", ev.Provenance.File, readErr)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != ev.Provenance.Hash {
			t.Errorf("provenance hash mismatch for %s: got %s want %s", ev.Provenance.File, got, ev.Provenance.Hash)
		}
	}
}
