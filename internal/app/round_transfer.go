package app

import (
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"os"
	"sort"
	"strconv"
	"strings"
)

const maxRoundPageSize = 50

// RoundReceipt is the bounded response returned by a successful Stop command.
type RoundReceipt struct {
	ID            string `json:"id"`
	Session       string `json:"session"`
	Started       string `json:"started"`
	Ended         string `json:"ended"`
	ChangeCount   int    `json:"change_count"`
	AddedCount    int    `json:"added_count"`
	ModifiedCount int    `json:"modified_count"`
	DeletedCount  int    `json:"deleted_count"`
	OmittedCount  int    `json:"omitted_count"`
	Source        string `json:"source"`
}

func ReceiptForRound(round Round) RoundReceipt {
	receipt := RoundReceipt{ID: round.ID, Session: round.Session, Started: round.Started, Ended: round.Ended, ChangeCount: len(round.Changes), OmittedCount: len(round.Omitted), Source: round.Source}
	for _, change := range round.Changes {
		switch change.Kind {
		case "added":
			receipt.AddedCount++
		case "deleted":
			receipt.DeletedCount++
		default:
			receipt.ModifiedCount++
		}
	}
	return receipt
}

type RoundSummary struct {
	ID            string `json:"id"`
	Session       string `json:"session"`
	Started       string `json:"started"`
	Ended         string `json:"ended"`
	ChangeCount   int    `json:"change_count"`
	AddedCount    int    `json:"added_count"`
	ModifiedCount int    `json:"modified_count"`
	DeletedCount  int    `json:"deleted_count"`
	OmittedCount  int    `json:"omitted_count"`
	Source        string `json:"source"`
}

type RoundPage struct {
	Items      []RoundSummary `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

func summaryForRound(round Round) RoundSummary {
	receipt := ReceiptForRound(round)
	return RoundSummary(receipt)
}

// ListPage returns compact summaries. The decimal cursor is an opaque offset
// into the current newest-first listing and each response contains at most 50.
func (a *App) ListPage(cursor string, limit int) (RoundPage, error) {
	if err := a.scrubExcluded(); err != nil {
		return RoundPage{}, err
	}
	if limit <= 0 {
		limit = maxRoundPageSize
	}
	if limit > maxRoundPageSize {
		limit = maxRoundPageSize
	}
	offset := 0
	if cursor != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed < 0 {
			return RoundPage{}, errors.New("无效轮次分页游标")
		}
		offset = parsed
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return RoundPage{}, err
	}
	all := make([]RoundSummary, 0)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "round-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var round Round
		if err := a.read(entry.Name(), &round); err != nil {
			return RoundPage{}, err
		}
		all = append(all, summaryForRound(round))
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Started == all[j].Started {
			return all[i].ID > all[j].ID
		}
		return all[i].Started > all[j].Started
	})
	if offset > len(all) {
		return RoundPage{}, errors.New("轮次分页游标超出范围")
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	page := RoundPage{Items: append([]RoundSummary(nil), all[offset:end]...)}
	if end < len(all) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

type RoundFileSummary struct {
	Hash  string `json:"hash"`
	Bytes int    `json:"bytes"`
}

type RoundChangeDetail struct {
	Path   string            `json:"path"`
	Kind   string            `json:"kind"`
	Before *RoundFileSummary `json:"before,omitempty"`
	After  *RoundFileSummary `json:"after,omitempty"`
	Folded bool              `json:"folded"`
}

type RoundDetail struct {
	ID           string               `json:"id"`
	Session      string               `json:"session"`
	Started      string               `json:"started"`
	Ended        string               `json:"ended"`
	Changes      []RoundChangeDetail  `json:"changes"`
	OmittedCount int                  `json:"omitted_count"`
	Notes        []string             `json:"notes"`
	Source       string               `json:"source"`
	Analysis     *RoundAnalysisDetail `json:"analysis,omitempty"`
	NextCursor   string               `json:"next_cursor,omitempty"`
}

type RoundAnalysisDetail struct {
	ID          string             `json:"id"`
	File        string             `json:"file"`
	FunctionID  string             `json:"function_id"`
	Created     string             `json:"created"`
	Document    explain.Document   `json:"document"`
	Evidence    []explain.Evidence `json:"evidence"`
	Freshness   string             `json:"freshness"`
	Corrections []Correction       `json:"corrections,omitempty"`
}

func fileSummary(file *capture.File) *RoundFileSummary {
	if file == nil {
		return nil
	}
	return &RoundFileSummary{Hash: file.Hash, Bytes: len(file.Content)}
}

// RoundDetails returns metadata and evidence descriptors without source text.
// Source bodies are read through RoundFile or ExplanationEvidenceByID.
func (a *App) RoundDetails(id, cursor string, limit int) (RoundDetail, error) {
	if !validID(id) {
		return RoundDetail{}, errors.New("无效轮次 ID")
	}
	if err := a.scrubExcluded(); err != nil {
		return RoundDetail{}, err
	}
	var round Round
	if err := a.read("round-"+id+".json", &round); err != nil {
		return RoundDetail{}, err
	}
	if round.ID != id {
		return RoundDetail{}, errors.New("轮次 ID 不匹配")
	}
	if limit <= 0 {
		limit = maxRoundPageSize
	}
	if limit > maxRoundPageSize {
		limit = maxRoundPageSize
	}
	offset := 0
	if cursor != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed < 0 {
			return RoundDetail{}, errors.New("无效轮次详情分页游标")
		}
		offset = parsed
	}
	if offset > len(round.Changes) {
		return RoundDetail{}, errors.New("轮次详情分页游标超出范围")
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return RoundDetail{}, err
	}
	analysisRounds := []Round{round}
	if err := a.attachRoundAnalyses(analysisRounds, entries); err != nil {
		return RoundDetail{}, err
	}
	round = analysisRounds[0]
	end := offset + limit
	if end > len(round.Changes) {
		end = len(round.Changes)
	}
	detail := RoundDetail{ID: round.ID, Session: round.Session, Started: round.Started, Ended: round.Ended, OmittedCount: len(round.Omitted), Notes: round.Notes, Source: round.Source, Changes: make([]RoundChangeDetail, 0, end-offset)}
	for _, change := range round.Changes[offset:end] {
		detail.Changes = append(detail.Changes, RoundChangeDetail{Path: change.Path, Kind: change.Kind, Before: fileSummary(change.Before), After: fileSummary(change.After), Folded: change.Folded})
	}
	if end < len(round.Changes) {
		detail.NextCursor = strconv.Itoa(end)
	}
	if round.Analysis != nil {
		analysis := *round.Analysis
		for index := range analysis.Evidence {
			analysis.Evidence[index].Source = ""
		}
		detail.Analysis = &RoundAnalysisDetail{ID: analysis.ID, File: analysis.File, FunctionID: analysis.FunctionID, Created: analysis.Created, Document: analysis.Document, Evidence: analysis.Evidence, Freshness: analysis.Freshness, Corrections: analysis.Corrections}
	}
	return detail, nil
}

type RoundFileResult struct {
	Exists bool          `json:"exists"`
	File   *capture.File `json:"file,omitempty"`
}

// RoundFile reads a single before/after file. Capture bounds it to 1 MiB;
// even JSON's worst-case escaping stays well below the extension's 32 MiB cap.
func (a *App) RoundFile(id, relative, side string) (RoundFileResult, error) {
	if !validID(id) {
		return RoundFileResult{}, errors.New("无效轮次 ID")
	}
	if side != "before" && side != "after" {
		return RoundFileResult{}, errors.New("快照 side 必须为 before 或 after")
	}
	if err := a.scrubExcluded(); err != nil {
		return RoundFileResult{}, err
	}
	var round Round
	if err := a.read("round-"+id+".json", &round); err != nil {
		return RoundFileResult{}, err
	}
	for _, change := range round.Changes {
		if change.Path != relative {
			continue
		}
		file := change.Before
		if side == "after" {
			file = change.After
		}
		if file == nil {
			return RoundFileResult{Exists: false}, nil
		}
		if len(file.Content) > capture.MaxFileBytes {
			return RoundFileResult{}, fmt.Errorf("快照文件超过 %d 字节传输上限", capture.MaxFileBytes)
		}
		return RoundFileResult{Exists: true, File: file}, nil
	}
	return RoundFileResult{}, errors.New("指定文件不属于此轮次或已被当前排除规则移除")
}

func (a *App) ExplanationEvidenceByID(explanationID, evidenceID string) (explain.Evidence, error) {
	explanation, err := a.ExplanationByID(explanationID)
	if err != nil {
		return explain.Evidence{}, err
	}
	for _, evidence := range explanation.Evidence {
		if evidence.ID != evidenceID {
			continue
		}
		allowed, err := a.evidenceAllowed(evidence)
		if err != nil {
			return explain.Evidence{}, err
		}
		if !allowed {
			return explain.Evidence{}, errors.New("引用证据已被当前排除策略撤销")
		}
		if len(evidence.Source) > capture.MaxFileBytes {
			return explain.Evidence{}, errors.New("引用证据超过单次读取上限")
		}
		return evidence, nil
	}
	return explain.Evidence{}, errors.New("解释中不存在指定引用证据")
}
