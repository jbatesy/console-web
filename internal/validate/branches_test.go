package validate_test

import (
	"strings"
	"testing"

	"console-web/internal/db"
	"console-web/internal/validate"
)

func TestResolveCommand_LegacyTemplateOnly(t *testing.T) {
	cmd := db.Command{Label: "Hi", Template: "echo hello"}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || tmpl != "echo hello" {
		t.Errorf("got (%q, %v)", tmpl, ok)
	}
}

func TestResolveCommand_LegacyEmptyTemplateSkipped(t *testing.T) {
	cmd := db.Command{Label: "Hi", Template: ""}
	_, ok, err := validate.ResolveCommand(cmd, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected skip for empty legacy template")
	}
}

func TestResolveCommand_FirstMatchWins(t *testing.T) {
	cmd := db.Command{
		Label: "Deploy",
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$"}, Template: "prod-cmd"},
			{When: map[string]string{"env": "^staging$"}, Template: "staging-cmd"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "staging"})
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if tmpl != "staging-cmd" {
		t.Errorf("got %q", tmpl)
	}
}

func TestResolveCommand_DefaultBranch(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$"}, Template: "prod-cmd"},
			{Default: true, Template: "default-cmd"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "dev"})
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if tmpl != "default-cmd" {
		t.Errorf("got %q", tmpl)
	}
}

func TestResolveCommand_FirstDefaultWinsWhenMultiple(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{Default: true, Template: "first-default"},
			{Default: true, Template: "second-default"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{})
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if tmpl != "first-default" {
		t.Errorf("got %q, want first-default", tmpl)
	}
}

func TestResolveCommand_DefaultDeferred(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{Default: true, Template: "default-cmd"},
			{When: map[string]string{"env": "^prod$"}, Template: "prod-cmd"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "prod"})
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if tmpl != "prod-cmd" {
		t.Errorf("expected prod-cmd, got %q", tmpl)
	}
}

func TestResolveCommand_ANDWhen(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$", "region": "^us$"}, Template: "match"},
			{Default: true, Template: "fallback"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "prod", "region": "eu"})
	if err != nil || !ok || tmpl != "fallback" {
		t.Errorf("expected fallback when AND fails, got (%q, %v, %v)", tmpl, ok, err)
	}

	tmpl, ok, err = validate.ResolveCommand(cmd, map[string]string{"env": "prod", "region": "us"})
	if err != nil || !ok || tmpl != "match" {
		t.Errorf("got (%q, %v, %v)", tmpl, ok, err)
	}
}

func TestResolveCommand_NoMatchSkipped(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$"}, Template: "prod-cmd"},
		},
	}
	_, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected skip when no branch matches")
	}
}

func TestResolveCommand_EmptyWhenNeverMatches(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{}, Template: "should-not-run"},
		},
	}
	_, ok, err := validate.ResolveCommand(cmd, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected skip for non-default branch with empty when")
	}
}

func TestResolveCommand_BarePatternRequiresFullMatch(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "prod"}, Template: "prod-cmd"},
			{Default: true, Template: "default-cmd"},
		},
	}
	tmpl, ok, err := validate.ResolveCommand(cmd, map[string]string{"env": "preprod"})
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if tmpl != "default-cmd" {
		t.Errorf("bare pattern must not substring-match; got %q", tmpl)
	}

	tmpl, ok, err = validate.ResolveCommand(cmd, map[string]string{"env": "prod"})
	if err != nil || !ok || tmpl != "prod-cmd" {
		t.Errorf("expected prod-cmd, got (%q, %v, %v)", tmpl, ok, err)
	}
}

func TestCommand_ValidLegacy(t *testing.T) {
	if err := validate.Command(db.Command{Template: "echo hi"}, nil); err != nil {
		t.Error(err)
	}
}

func TestCommand_LegacyRequiresTemplate(t *testing.T) {
	err := validate.Command(db.Command{Label: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "template is required") {
		t.Errorf("expected template error, got %v", err)
	}
}

func TestCommand_ValidBranches(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$"}, Template: "prod"},
			{Default: true, Template: "default"},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^(prod|staging|dev)$`}}
	if err := validate.Command(cmd, vars); err != nil {
		t.Error(err)
	}
}

func TestCommand_BranchTemplateRequired(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "^prod$"}, Template: ""},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^prod$`}}
	err := validate.Command(cmd, vars)
	if err == nil || !strings.Contains(err.Error(), "template is required") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_InvalidRegex(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": "[invalid"}, Template: "x"},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^prod$`}}
	err := validate.Command(cmd, vars)
	if err == nil || !strings.Contains(err.Error(), "invalid regex") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_AtMostOneDefault(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{Default: true, Template: "a"},
			{Default: true, Template: "b"},
		},
	}
	err := validate.Command(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "at most one default") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_DefaultMustNotHaveWhen(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{Default: true, When: map[string]string{"env": "^prod$"}, Template: "x"},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^prod$`}}
	err := validate.Command(cmd, vars)
	if err == nil || !strings.Contains(err.Error(), "default branch must not have when") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_EmptyWhenPattern(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"env": ""}, Template: "x"},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^prod$`}}
	err := validate.Command(cmd, vars)
	if err == nil || !strings.Contains(err.Error(), "when pattern for \"env\" must not be empty") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_NonDefaultRequiresWhen(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{Template: "x"},
		},
	}
	err := validate.Command(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "must have at least one when entry") {
		t.Errorf("got %v", err)
	}
}

func TestCommand_UnknownWhenVariable(t *testing.T) {
	cmd := db.Command{
		Branches: []db.CommandBranch{
			{When: map[string]string{"region": "^us$"}, Template: "x"},
		},
	}
	vars := []db.Variable{{Name: "env", Regex: `^prod$`}}
	err := validate.Command(cmd, vars)
	if err == nil || !strings.Contains(err.Error(), "unknown variable \"region\"") {
		t.Errorf("got %v", err)
	}
}
