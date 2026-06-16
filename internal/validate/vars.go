package validate

import (
	"regexp"
	"strings"

	"console-web/internal/db"
)

type Error struct {
	Name    string
	Value   string
	Pattern string
}

// Vars validates each job variable against the supplied vars map.
// Returns one Error per failing variable. Variables defined in the job
// but absent from vars are treated as empty string and validated normally.
func Vars(job *db.Job, vars map[string]string) []Error {
	var errs []Error
	for _, v := range job.Variables {
		val := vars[v.Name]
		matched, err := regexp.MatchString(v.Regex, val)
		if err != nil || !matched {
			errs = append(errs, Error{Name: v.Name, Value: val, Pattern: v.Regex})
		}
	}
	return errs
}

// Substitute replaces all {{name}} placeholders in template with values from vars.
func Substitute(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
