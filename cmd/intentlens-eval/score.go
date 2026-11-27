package main

import (
	"fmt"
	"sort"
	"strings"

	"intentlens/internal/explain"
)

// Rule matches a claim when its text contains any keyword (case-insensitive).
type Rule struct {
	ID  string   `json:"id"`
	Any []string `json:"any"`
}

type Case struct {
	File      string         `json:"file"`
	Function  string         `json:"function"`
	Question  string         `json:"question,omitempty"`
	Expect    []Rule         `json:"expect"`
	Forbid    []Rule         `json:"forbid"`
	Selection *SelectionCase `json:"selection,omitempty"`
}

type SelectionMode string

const (
	SelectionModeOnly    SelectionMode = "selection_only"
	SelectionModeRelated SelectionMode = "related"
)

type SelectionFreshness string

const (
	SelectionFreshnessHit     SelectionFreshness = "hit"
	SelectionFreshnessUnknown SelectionFreshness = "unknown"
	SelectionFreshnessStale   SelectionFreshness = "stale"
	SelectionFreshnessMissing SelectionFreshness = "missing"
)

type SelectionFreshnessLabel string

const (
	SelectionLabelVerifiedSnapshot SelectionFreshnessLabel = "verified_at_snapshot"
	SelectionLabelUnverified       SelectionFreshnessLabel = "freshness_unverified"
	SelectionLabelStale            SelectionFreshnessLabel = "stale"
	SelectionLabelMissing          SelectionFreshnessLabel = "missing"
)

type SelectionRefreshHint string

const (
	SelectionHintNone            SelectionRefreshHint = "none"
	SelectionHintPrescanRequired SelectionRefreshHint = "prescan_required"
	SelectionHintExplicitRefresh SelectionRefreshHint = "explicit_refresh"
)

type SelectionCodeAnchorStatus string

const (
	SelectionAnchorSufficient    SelectionCodeAnchorStatus = "sufficient"
	SelectionAnchorInsufficient  SelectionCodeAnchorStatus = "insufficient"
	SelectionAnchorNotApplicable SelectionCodeAnchorStatus = "not_applicable"
)

type SelectionEvidenceType string

const (
	SelectionEvidenceCode       SelectionEvidenceType = "selected_code"
	SelectionEvidenceDiagnostic SelectionEvidenceType = "diagnostic"
	SelectionEvidenceRelation   SelectionEvidenceType = "code_relation"
	SelectionEvidenceDocument   SelectionEvidenceType = "documentation"
)

// D08 evidence tiers are separate from a model claim's support status.
type SelectionSupportLevel string

const (
	SelectionTierResolvedDirect SelectionSupportLevel = "resolved_direct"
	SelectionTierNameCandidate  SelectionSupportLevel = "name_candidate"
	SelectionTierDocMention     SelectionSupportLevel = "doc_mention"
	SelectionTierDocDecision    SelectionSupportLevel = "doc_decision"
)

type SelectionClaimStatus string

const (
	SelectionClaimSupported   SelectionClaimStatus = "supported"
	SelectionClaimUncertain   SelectionClaimStatus = "uncertain"
	SelectionClaimUnsupported SelectionClaimStatus = "unsupported"
)

type SelectionForbidKind string

const (
	SelectionForbidUnselectedFullText SelectionForbidKind = "unselected_full_text"
	SelectionForbidAbsolutePath       SelectionForbidKind = "absolute_path"
	SelectionForbidExcludedContent    SelectionForbidKind = "excluded_content"
	SelectionForbidPrescanRelation    SelectionForbidKind = "prescan_relation"
	SelectionForbidCandidateAsFact    SelectionForbidKind = "candidate_as_fact"
	SelectionForbidUnapprovedEvidence SelectionForbidKind = "unapproved_evidence"
	SelectionForbidUnanchoredCode     SelectionForbidKind = "unanchored_code_relation"
)

// SelectionCase is the frozen eval contract. Run a separate directory with
// Set INTENTLENS_API_KEY, then run `intentlens-eval -model <model> -cases eval/explain/selection-cases -only selection -out eval-out-selection`.
// so the existing function-case score baseline stays intact. In review.csv,
// use the notes column for selection-specific observations: evidence
// completeness (expected vs returned), support-tier/status correctness,
// provenance completeness, forbid violation count across every displayed claim
// regardless of status, and D04 freshness state/label/refresh handling. These
// are review metrics only until step 8b; current CSV scoring remains
// function-case-only.
type SelectionCase struct {
	Mode                    SelectionMode           `json:"mode"`
	Freshness               SelectionFreshness      `json:"freshness"`
	FreshnessLabel          SelectionFreshnessLabel `json:"freshness_label"`
	RefreshHint             SelectionRefreshHint    `json:"refresh_hint"`
	Authorization           SelectionAuthorization  `json:"authorization"`
	Range                   SelectionRange          `json:"range"`
	Diagnostics             []SelectionDiagnostic   `json:"diagnostics,omitempty"`
	AuthorizedEvidenceTypes []SelectionEvidenceType `json:"authorized_evidence_types"`
	ExpectedEvidence        []SelectionEvidence     `json:"expected_evidence"`
	Forbid                  []SelectionForbid       `json:"forbid"`
	PersistResult           bool                    `json:"persist_result"`
	PrescanCache            SelectionPrescanCache   `json:"prescan_cache"`
	PendingVariants         []SelectionVariant      `json:"pending_variants,omitempty"`
}

// SelectionVariant reserves executable related/unknown/stale contracts for 8b.
type SelectionVariant struct {
	Name                    string                    `json:"name"`
	Mode                    SelectionMode             `json:"mode"`
	Freshness               SelectionFreshness        `json:"freshness"`
	FreshnessLabel          SelectionFreshnessLabel   `json:"freshness_label"`
	RefreshHint             SelectionRefreshHint      `json:"refresh_hint"`
	Authorization           SelectionAuthorization    `json:"authorization"`
	CodeAnchorStatus        SelectionCodeAnchorStatus `json:"code_anchor_status,omitempty"`
	RefreshHintRequired     bool                      `json:"refresh_hint_required"`
	AuthorizedEvidenceTypes []SelectionEvidenceType   `json:"authorized_evidence_types"`
	ExpectedEvidence        []SelectionEvidence       `json:"expected_evidence"`
	Forbid                  []SelectionForbid         `json:"forbid"`
	PersistResult           bool                      `json:"persist_result"`
	PrescanCache            SelectionPrescanCache     `json:"prescan_cache"`
}

// SelectionAuthorization keeps relationship-send consent and its recipient
// binding separate from local prescan-retention consent.
type SelectionAuthorization struct {
	RelatedSendAuthorized bool   `json:"related_send_authorized"`
	Provider              string `json:"provider"`
	BaseURL               string `json:"base_url"`
	Version               string `json:"version"`
}

// Prescan retention is a separate data category from transient explanation results.
type SelectionPrescanCache struct {
	Retained                      bool `json:"retained"`
	Authorized                    bool `json:"authorized"`
	SeparateAuthorizationRequired bool `json:"separate_authorization_required"`
}

// SelectionRange uses zero-based, end-exclusive UTF-16 editor coordinates.
type SelectionRange struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
	EndLine     int `json:"end_line"`
	EndColumn   int `json:"end_column"`
}

// SelectionDiagnostic is a fixture diagnostic at editor coordinates; only
// intersecting Error entries are expected to enter a default selection request.
type SelectionDiagnostic struct {
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Range    SelectionRange `json:"range"`
}

// SelectionEvidence records expected evidence and local provenance. The
// provenance file is relative to the case, never a model-facing path.
type SelectionEvidence struct {
	ID               string                `json:"id"`
	Type             SelectionEvidenceType `json:"type"`
	SupportTier      SelectionSupportLevel `json:"support_tier,omitempty"`
	ClaimStatus      SelectionClaimStatus  `json:"claim_status,omitempty"`
	SnapshotTemporal bool                  `json:"snapshot_temporal,omitempty"`
	Provenance       SelectionProvenance   `json:"provenance"`
}

// SelectionProvenance uses a case-relative file, lowercase SHA-256 of its bytes,
// and 1-based inclusive source lines. It is local eval metadata, never model input.
type SelectionProvenance struct {
	File      string `json:"file"`
	Hash      string `json:"hash"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type SelectionForbid struct {
	ID          string                `json:"id"`
	Kind        SelectionForbidKind   `json:"kind"`
	SupportTier SelectionSupportLevel `json:"support_tier,omitempty"`
	ClaimStatus SelectionClaimStatus  `json:"claim_status,omitempty"`
	Canary      string                `json:"canary,omitempty"`
}

type Score struct {
	Claims           int            `json:"claims"`
	Statuses         map[string]int `json:"statuses"`
	InvalidCitations []string       `json:"invalid_citations,omitempty"`
	Uncited          []string       `json:"uncited,omitempty"`
	ExpectMet        []string       `json:"expect_met"`
	ExpectMissed     []string       `json:"expect_missed"`
	ForbidHit        []string       `json:"forbid_hit,omitempty"`
}

func matches(r Rule, text string) bool {
	text = strings.ToLower(text)
	for _, k := range r.Any {
		if k != "" && strings.Contains(text, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

// score checks citations structurally. Keyword checks are a coarse first
// pass; the review sheet holds the human judgment.
func score(c Case, doc explain.Document, evidence []explain.Evidence) Score {
	ids := map[string]bool{}
	for _, ev := range evidence {
		ids[ev.ID] = true
	}
	s := Score{Statuses: map[string]int{}, ExpectMet: []string{}, ExpectMissed: []string{}}
	var supported []string
	for _, cl := range doc.Claims {
		s.Claims++
		s.Statuses[cl.Status]++
		if len(cl.EvidenceIDs) == 0 {
			s.Uncited = append(s.Uncited, cl.ID)
		}
		for _, id := range cl.EvidenceIDs {
			if !ids[id] {
				s.InvalidCitations = append(s.InvalidCitations, cl.ID+"->"+id)
			}
		}
		if cl.Status == "supported" {
			supported = append(supported, cl.Text)
		}
	}
	for _, r := range c.Expect {
		met := false
		for _, t := range supported {
			if matches(r, t) {
				met = true
				break
			}
		}
		if met {
			s.ExpectMet = append(s.ExpectMet, r.ID)
		} else {
			s.ExpectMissed = append(s.ExpectMissed, r.ID)
		}
	}
	for _, r := range c.Forbid {
		for _, t := range supported {
			if matches(r, t) {
				s.ForbidHit = append(s.ForbidHit, r.ID)
				break
			}
		}
	}
	return s
}

func evidenceRefs(ids []string, evidence []explain.Evidence) string {
	byID := map[string]explain.Evidence{}
	for _, ev := range evidence {
		byID[ev.ID] = ev
	}
	var refs []string
	for _, id := range ids {
		if ev, ok := byID[id]; ok && ev.File != "" {
			refs = append(refs, fmt.Sprintf("%s:%d-%d", ev.File, ev.StartLine, ev.EndLine))
		} else {
			refs = append(refs, id)
		}
	}
	sort.Strings(refs)
	return strings.Join(refs, " ")
}
