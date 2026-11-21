package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"intentlens/internal/explain"
)

func TestNewlyAllowedFileIsUnknownRatherThanAdded(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "existing.go", "package p\nfunc Existing() {}\n")
	put(t, root, ".intentignore", "existing.go\n")
	if _, e := a.Start("s"); e != nil {
		t.Fatal(e)
	}
	put(t, root, ".intentignore", "")
	r, e := a.Stop("s")
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Changes) != 0 {
		t.Fatalf("previously unobserved source reported added: %+v", r.Changes)
	}
	if len(r.Omitted) == 0 {
		t.Fatal("missing baseline not reported")
	}
}

func configurePolicyTestModel(t *testing.T, a *App) {
	t.Helper()
	cfg := explain.Config{
		Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid/v1",
		Model: "fixture", TokenLimit: 1000000, VerificationMode: explain.ModeStrict,
	}
	if err := a.Configure(Settings{Model: cfg}, true); err != nil {
		t.Fatal(err)
	}
}

func TestExplainRequestPolicyAndSettlement(t *testing.T) {
	const answerTokens, reviewTokens = 17, 23
	for _, tc := range []struct {
		name                  string
		revoke                string
		wantCalls, wantTokens int
	}{
		{name: "before first send", revoke: "before answer"},
		{name: "before review send", revoke: "after answer", wantCalls: 1, wantTokens: answerTokens},
		{name: "two stages allowed", wantCalls: 2, wantTokens: answerTokens + reviewTokens},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, root := fixture(t)
			put(t, root, "main.go", "package p\nfunc F() int { return Helper() }\n")
			put(t, root, "helper.go", "package p\nfunc Helper() int { return 1 }\n")
			configurePolicyTestModel(t, a)
			initial := explain.Usage{ActualTokens: 11, UncertainTokens: 3}
			if err := a.write("usage.json", initial); err != nil {
				t.Fatal(err)
			}
			revoke := func() { put(t, root, ".intentignore", "helper.go\n") }
			if tc.revoke == "before answer" {
				a.afterModelReserve = revoke
			}
			calls := 0
			a.modelTransport = modelConnectionTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				var body struct {
					Messages []struct{ Content string }
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if len(body.Messages) != 2 {
					t.Fatalf("unexpected message count: %d", len(body.Messages))
				}
				var prompt struct {
					Evidence []explain.Evidence
					Claims   []explain.Claim
				}
				if err := json.Unmarshal([]byte(body.Messages[1].Content), &prompt); err != nil {
					t.Fatal(err)
				}
				var answer string
				var tokens int
				switch calls {
				case 1:
					if len(prompt.Evidence) != 2 || prompt.Evidence[0].File != "main.go" || prompt.Evidence[1].File != "helper.go" {
						t.Fatalf("expected function and dependency in first request: %+v", prompt.Evidence)
					}
					// Claim text can echo an uncited source. Revoke the dependency,
					// even though only main.go appears in the review evidence list.
					doc := explain.Document{Claims: []explain.Claim{{
						ID: "c1", Text: "func Helper() int { return 1 }", Kind: "behavior",
						EvidenceIDs: []string{prompt.Evidence[0].ID},
					}}}
					encoded, err := json.Marshal(doc)
					if err != nil {
						t.Fatal(err)
					}
					if tc.revoke == "after answer" {
						revoke()
					}
					answer, tokens = string(encoded), answerTokens
				case 2:
					if len(prompt.Claims) != 1 || len(prompt.Evidence) != 1 || prompt.Evidence[0].File != "main.go" {
						t.Fatalf("unexpected review payload: %+v", prompt)
					}
					answer, tokens = `{"checks":[{"claim_id":"c1","status":"supported"}]}`, reviewTokens
				default:
					t.Fatalf("unexpected request %d", calls)
				}
				response := httptest.NewRecorder()
				if err := json.NewEncoder(response).Encode(map[string]any{
					"choices": []any{map[string]any{"message": map[string]string{"content": answer}, "finish_reason": "stop"}},
					"usage":   map[string]int{"total_tokens": tokens},
				}); err != nil {
					t.Fatal(err)
				}
				return response.Result(), nil
			})
			saved, err := a.Explain(context.Background(), "main.go", 2, "", "key")
			if calls != tc.wantCalls {
				t.Fatalf("made %d requests, want %d (err=%v)", calls, tc.wantCalls, err)
			}
			records, globErr := filepath.Glob(filepath.Join(a.State, "explanation-*.json"))
			if globErr != nil {
				t.Fatal(globErr)
			}
			if tc.revoke != "" {
				if !errors.Is(err, explain.ErrRequestBlocked) || !strings.Contains(err.Error(), "helper.go") || saved.ID != "" || len(records) != 0 {
					t.Fatalf("expected blocked request and no stored result: result=%+v records=%v err=%v", saved, records, err)
				}
			} else if err != nil || len(records) != 1 || len(saved.Document.Claims) != 1 || saved.Document.Claims[0].Status != "supported" {
				t.Fatalf("expected a stored and reviewed result: result=%+v records=%v err=%v", saved, records, err)
			}
			assertModelUsage(t, a, explain.Usage{ActualTokens: initial.ActualTokens + tc.wantTokens, UncertainTokens: initial.UncertainTokens})
		})
	}
}

func TestRoundAndSelectionPolicyChangeAfterReservation(t *testing.T) {
	for _, kind := range []string{"round", "selection"} {
		t.Run(kind, func(t *testing.T) {
			a, root := fixture(t)
			configurePolicyTestModel(t, a)
			const file = "main.go"
			const source = "package p"
			a.afterModelReserve = func() { put(t, root, ".intentignore", file+"\n") }
			assertNoModelRequest(t, a)
			ctx := context.Background()
			var err error
			switch kind {
			case "round":
				round := Round{ID: newID(), Changes: []capture.Change{{Path: file, After: &capture.File{Content: source}}}}
				_, err = a.ExplainRound(ctx, round, codeindex.Index{}, "key")
			case "selection":
				r := explain.EditorRange{End: explain.EditorPosition{Character: len(source)}}
				_, err = a.ExplainSelection(ctx, file, source, "go", 1, 1, 1, r, nil, "key")
			}
			if !errors.Is(err, explain.ErrRequestBlocked) || !strings.Contains(err.Error(), file) {
				t.Fatalf("expected policy rejection for %s: %v", file, err)
			}
			assertModelUsage(t, a, explain.Usage{})
		})
	}
}

func TestExerciseModelRequestPolicyAfterReservation(t *testing.T) {
	t.Run("generation checks the whole package baseline", func(t *testing.T) {
		a, root := fixture(t)
		source := seedExplanation(t, a, root)
		put(t, root, "helper.go", "package p\nfunc Helper() int { return 1 }\n")
		a.afterModelReserve = func() { put(t, root, ".intentignore", "helper.go\n") }
		assertNoModelRequest(t, a)
		generated, err := a.GenerateExercise(context.Background(), source.ID, "c1", "main.go", "", "key")
		if !errors.Is(err, explain.ErrRequestBlocked) || !strings.Contains(err.Error(), "helper.go") || generated.ID != "" {
			t.Fatalf("expected blocked exercise generation: result=%+v err=%v", generated, err)
		}
		records, globErr := filepath.Glob(filepath.Join(a.State, "exercise-*.json"))
		if globErr != nil || len(records) != 0 {
			t.Fatalf("exercise record created: %v (err=%v)", records, globErr)
		}
		assertModelUsage(t, a, explain.Usage{})
	})

	t.Run("feedback checks submitted files", func(t *testing.T) {
		a, root := fixture(t)
		e := seedExercise(t, a, root)
		attempt := ExerciseAttempt{ID: newID(), Version: e.Version, Files: map[string]string{
			"answer.go": "package p\nfunc Answer() int { return 2 }\n",
		}}
		attemptsFile := "exercise-attempts-" + e.ID + ".json"
		if err := a.write(attemptsFile, []ExerciseAttempt{attempt}); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(a.State, attemptsFile))
		if err != nil {
			t.Fatal(err)
		}
		a.exerciseRun = nil
		a.afterModelReserve = func() { put(t, root, ".intentignore", "answer.go\n") }
		assertNoModelRequest(t, a)
		_, err = a.ExerciseFeedback(context.Background(), e.ID, attempt.ID, "修改理由", "key")
		if !errors.Is(err, explain.ErrRequestBlocked) || !strings.Contains(err.Error(), "answer.go") {
			t.Fatalf("expected blocked exercise feedback: %v", err)
		}
		after, err := os.ReadFile(filepath.Join(a.State, attemptsFile))
		if err != nil || string(before) != string(after) {
			t.Fatalf("blocked feedback changed the saved attempt: %v", err)
		}
		assertModelUsage(t, a, explain.Usage{ActualTokens: 10})
	})
}

func assertNoModelRequest(t *testing.T, a *App) {
	t.Helper()
	a.modelTransport = modelConnectionTransport(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected model request to %s", req.URL)
		return nil, errors.New("unexpected model request")
	})
}

func assertModelUsage(t *testing.T, a *App, want explain.Usage) {
	t.Helper()
	got, err := a.Usage()
	if err != nil || got != want {
		t.Fatalf("usage = %+v, want %+v (err=%v)", got, want, err)
	}
}
