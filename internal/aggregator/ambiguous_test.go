package aggregator

import "testing"

func TestParseAmbiguousCallMessage(t *testing.T) {
tests := []struct {
name    string
message string
want    string
}{
{
name:    "Chinese format",
message: "调用目标未唯一确定：errors.New",
want:    "errors.New",
},
{
name:    "English format",
message: "ambiguous reference to New",
want:    "New",
},
{
name:    "Generic ambiguous",
message: "ambiguous call",
want:    "ambiguous",
},
{
name:    "Not ambiguous",
message: "some other error",
want:    "",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
got := parseAmbiguousCallMessage(tt.message)
if got != tt.want {
t.Errorf("parseAmbiguousCallMessage() = %v, want %v", got, tt.want)
}
})
}
}

func TestSummarizeAmbiguousCalls(t *testing.T) {
tests := []struct {
name               string
restrictions       []Restriction
wantCount          int
wantSampleCount    int
wantMessageContains string
}{
{
name: "single ambiguous call",
restrictions: []Restriction{
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：errors.New"},
},
wantCount:          1,
wantSampleCount:    1,
wantMessageContains: "1 ambiguous call",
},
{
name: "multiple ambiguous calls",
restrictions: []Restriction{
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：errors.New"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：len"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：json.Valid"},
},
wantCount:          3,
wantSampleCount:    3,
wantMessageContains: "3 ambiguous calls",
},
{
name: "many ambiguous calls",
restrictions: []Restriction{
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：errors.New"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：len"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：json.Valid"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：sha256.Sum256"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：hex.EncodeToString"},
},
wantCount:          5,
wantSampleCount:    3,
wantMessageContains: "5 ambiguous calls (e.g.,",
},
{
name: "deduplication",
restrictions: []Restriction{
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：errors.New"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：errors.New"},
{Category: "Ambiguous Calls", Message: "调用目标未唯一确定：len"},
},
wantCount:          2,
wantSampleCount:    2,
wantMessageContains: "2 ambiguous calls",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
summary := summarizeAmbiguousCalls(tt.restrictions)

if summary.Type != "summary" {
t.Errorf("Type = %v, want summary", summary.Type)
}

if summary.Count != tt.wantCount {
t.Errorf("Count = %v, want %v", summary.Count, tt.wantCount)
}

if len(summary.SampleItems) != tt.wantSampleCount {
t.Errorf("SampleItems length = %v, want %v", len(summary.SampleItems), tt.wantSampleCount)
}

if !contains(summary.Message, tt.wantMessageContains) {
t.Errorf("Message = %v, want to contain %v", summary.Message, tt.wantMessageContains)
}
})
}
}

func TestAggregateWithAmbiguousCalls(t *testing.T) {
issues := []string{
"调用目标未唯一确定：errors.New",
"调用目标未唯一确定：errors.New",
"调用目标未唯一确定：len",
"调用目标未唯一确定：errors.New",
"调用目标未唯一确定：json.Valid",
"调用目标未唯一确定：errors.New",
"调用目标未唯一确定：sha256.Sum256",
"调用目标未唯一确定：hex.EncodeToString",
"调用目标未唯一确定：uuid.NewString",
"调用目标未唯一确定：datatypes.JSON",
"调用目标未唯一确定：fmt.Errorf",
}

result := Aggregate(issues)

// Should have 1 category: Ambiguous Calls
if result.Summary.TotalCategories != 1 {
t.Errorf("TotalCategories = %v, want 1", result.Summary.TotalCategories)
}

// Total restrictions should be 11
if result.Summary.TotalRestrictions != 11 {
t.Errorf("TotalRestrictions = %v, want 11", result.Summary.TotalRestrictions)
}

// Find Ambiguous Calls category
var ambiguousGroup *CategoryGroup
for i := range result.Categories {
if result.Categories[i].Name == "Ambiguous Calls" {
ambiguousGroup = &result.Categories[i]
break
}
}

if ambiguousGroup == nil {
t.Fatal("Ambiguous Calls category not found")
}

// Should have 11 ambiguous calls
if ambiguousGroup.Count != 11 {
t.Errorf("Ambiguous Calls count = %v, want 11", ambiguousGroup.Count)
}

// Items should be a summary (not individual restrictions)
if len(ambiguousGroup.Items) != 1 {
t.Errorf("Ambiguous Calls items length = %v, want 1 (summary)", len(ambiguousGroup.Items))
}

// Verify it's a summary
if summary, ok := ambiguousGroup.Items[0].(ItemSummary); ok {
if summary.Type != "summary" {
t.Errorf("Item type = %v, want summary", summary.Type)
}

// Should have deduplicated to 8 unique targets (errors.New appears 4 times)
expectedUniqueCount := 8
if summary.Count != expectedUniqueCount {
t.Errorf("Summary count = %v, want %v (deduplicated)", summary.Count, expectedUniqueCount)
}

// Message should indicate multiple calls
if !contains(summary.Message, "ambiguous calls") {
t.Errorf("Summary message = %v, want to contain 'ambiguous calls'", summary.Message)
}

// Should show sample of first 3
if len(summary.SampleItems) != 3 {
t.Errorf("Sample items length = %v, want 3", len(summary.SampleItems))
}

// Full list should have all 8 unique targets
if len(summary.FullList) != expectedUniqueCount {
t.Errorf("Full list length = %v, want %v", len(summary.FullList), expectedUniqueCount)
}
} else {
t.Errorf("Item is not ItemSummary, got type %T", ambiguousGroup.Items[0])
}
}
