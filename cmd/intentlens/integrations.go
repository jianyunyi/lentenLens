package main

func integrationInfo() any {
	return map[string]any{
		"schema":                            1,
		"entries":                           []string{"ide", "skill", "claude"},
		"transport":                         "local-process-json-stdio",
		"root_flag":                         "--root <absolute-project-directory>",
		"model_required_for_local_evidence": false,
		"index_coverage": map[string]any{
			"responses":             []string{"index.coverage", "inspect.coverage", "call-path.coverage"},
			"fields":                []string{"complete", "scope", "budget_bytes", "captured_bytes", "omitted_count", "excluded_count", "omitted", "truncated"},
			"omitted_examples_max":  20,
			"excluded_paths_listed": false,
			"call_statuses":         []string{"resolved", "unresolved", "ambiguous"},
			"partial_index_warning": "coverage.complete=false means the index is partial; report scope and counts, and do not infer that a missing file or function does not exist",
			"call_status_warning":   "complete coverage does not mean every direct call has a unique target; preserve each calls[].status",
			"call_path_warning":     "call-path follows only resolved direct static edges; unresolved, ambiguous, interface, cross-package and dynamic calls remain boundary evidence",
		},
		"capture_event": map[string]any{
			"command": "capture-event", "events": []string{"start", "stop"},
			"input":                    map[string]string{"event": "start|stop", "session": "same non-empty unique ID for both events", "round_id": "round ID returned by start; send it on stop"},
			"start_response":           "includes the started round id",
			"failure":                  "nonzero exit and JSON error; never interpret a failed start as a baseline",
			"automatic_model_analysis": false,
		},
		"local_commands": []string{"start", "stop", "capture-status", "capture-abort", "rounds", "index", "inspect", "call-path", "status", "shared", "exercises", "exercise-view", "exercise-shared"},
		"model_commands": []string{"explain", "analyze-round", "revise", "share-prepare", "exercise-generate", "exercise-feedback", "test-model"},
		"round_analysis": "manual analyze-round --round <captured-round-id> (or stdin JSON {round_id: captured-round-id}) remains repeatable; lifecycle hooks use --automatic, which records completion per round ID and skips completed rounds; both require project model consent and budget",
		"claude_adapter": map[string]string{"configuration": "hooks-config", "event_command": "hook", "requirement": "optional; Claude lifecycle events only"},
		"limitations": []string{
			"IDE and Skill hosts must be able to invoke the local executable; their native lifecycle integrations are not auto-installed.",
			"Only saved files are captured; source attribution remains unconfirmed.",
			"Local capture and evidence do not require a model key; generated and checked explanations require opt-in model configuration.",
			"Skill-authored answers are not stored or checked IntentLens explanations.",
			"Sharing, execution and answer application retain their existing preview and confirmation requirements.",
		},
	}
}
