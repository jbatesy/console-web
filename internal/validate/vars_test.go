package validate_test

import (
	"testing"

	"console-web/internal/db"
	"console-web/internal/validate"
)

func TestValidate_AllPass(t *testing.T) {
	job := &db.Job{
		Variables: []db.Variable{
			{Name: "host", Regex: `^[\w.-]+$`},
			{Name: "port", Regex: `^\d{2,5}$`},
		},
	}
	errs := validate.Vars(job, map[string]string{"host": "myserver.local", "port": "8080"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %+v", errs)
	}
}

func TestValidate_Fails(t *testing.T) {
	job := &db.Job{
		Variables: []db.Variable{
			{Name: "host", Regex: `^[\w.-]+$`},
			{Name: "port", Regex: `^\d{2,5}$`},
		},
	}
	errs := validate.Vars(job, map[string]string{"host": "bad host!", "port": "999999"})
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %+v", len(errs), errs)
	}
	if errs[0].Name != "host" {
		t.Errorf("first error name: %q", errs[0].Name)
	}
}

func TestValidate_MissingVar(t *testing.T) {
	job := &db.Job{
		Variables: []db.Variable{
			{Name: "host", Regex: `^[\w.-]+$`},
		},
	}
	// not providing "host" should fail — empty string won't match
	errs := validate.Vars(job, map[string]string{})
	if len(errs) != 1 {
		t.Errorf("expected 1 error for missing var, got %d", len(errs))
	}
}

func TestSubstitute(t *testing.T) {
	result := validate.Substitute("curl -s {{host}}/status", map[string]string{"host": "myserver.local"})
	if result != "curl -s myserver.local/status" {
		t.Errorf("got %q", result)
	}
}
