package app

import "testing"

func TestInspectGenericDependenciesHonorExclusions(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc Caller() { Identity[int](1); (Helper)() }\n")
	put(t, root, "helper.go", "package p\nfunc Identity[T any](v T) T { return v }\nfunc Helper() {}\n")
	info, err := a.Inspect("main.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Dependencies) != 2 || len(info.Evidence) != 3 {
		t.Fatalf("direct dependency evidence missing: %+v", info)
	}
	for i, name := range []string{"Identity", "Helper"} {
		if info.Dependencies[i].Name != name || info.Evidence[i+1].File != "helper.go" || info.Evidence[i+1].Source != info.Dependencies[i].Source {
			t.Fatalf("wrong dependency evidence: %+v", info)
		}
	}
	put(t, root, ".intentignore", "helper.go\n")
	info, err = a.Inspect("main.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Dependencies) != 0 || len(info.Evidence) != 1 || info.Evidence[0].File != "main.go" {
		t.Fatalf("excluded dependency leaked: %+v", info)
	}
	for _, call := range info.Function.Calls {
		if call.Status != "unresolved" || len(call.Targets) != 0 {
			t.Fatalf("excluded target guessed: %+v", call)
		}
	}
}
