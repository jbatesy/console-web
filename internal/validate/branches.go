package validate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"console-web/internal/db"
)

// ErrNoMatchingCommands is returned when every command in a job is skipped at launch.
var ErrNoMatchingCommands = errors.New("no commands matched the supplied variables")

// ErrInvalidRegex is returned when a branch when-pattern fails to compile.
var ErrInvalidRegex = errors.New("invalid regex")

// ResolveCommand selects a template for cmd based on vars using first-match-wins
// branch evaluation. Returns ok=false when the command should be skipped.
func ResolveCommand(cmd db.Command, vars map[string]string) (template string, ok bool, err error) {
	if len(cmd.Branches) == 0 {
		return cmd.Template, cmd.Template != "", nil
	}

	var defaultBranch *db.CommandBranch
	for i := range cmd.Branches {
		b := &cmd.Branches[i]
		if b.Default {
			if defaultBranch == nil {
				defaultBranch = b
			}
			continue
		}
		matched, err := matchesAll(b.When, vars)
		if err != nil {
			return "", false, err
		}
		if matched {
			return b.Template, true, nil
		}
	}

	if defaultBranch != nil {
		return defaultBranch.Template, true, nil
	}
	return "", false, nil
}

func matchesAll(when map[string]string, vars map[string]string) (bool, error) {
	if len(when) == 0 {
		return false, nil
	}
	for varName, pattern := range when {
		re, err := regexp.Compile(fullMatchRegex(pattern))
		if err != nil {
			return false, fmt.Errorf("%w: pattern %q for variable %q: %v", ErrInvalidRegex, pattern, varName, err)
		}
		if !re.MatchString(vars[varName]) {
			return false, nil
		}
	}
	return true, nil
}

// Command validates a job command definition at save time.
// variables lists declared job variable names; when keys must reference them.
func Command(cmd db.Command, variables []db.Variable) error {
	var errs []string

	declared := make(map[string]struct{}, len(variables))
	for _, v := range variables {
		declared[v.Name] = struct{}{}
	}

	if len(cmd.Branches) == 0 {
		if cmd.Template == "" {
			errs = append(errs, "template is required when branches is empty")
		}
	} else {
		defaultCount := 0
		for i, b := range cmd.Branches {
			prefix := fmt.Sprintf("branch %d", i)
			if b.Template == "" {
				errs = append(errs, fmt.Sprintf("%s: template is required", prefix))
			}
			if b.Default {
				defaultCount++
				if len(b.When) > 0 {
					errs = append(errs, fmt.Sprintf("%s: default branch must not have when clauses", prefix))
				}
			} else if len(b.When) == 0 {
				errs = append(errs, fmt.Sprintf("%s: non-default branch must have at least one when entry", prefix))
			}
			for varName, pattern := range b.When {
				if _, ok := declared[varName]; !ok {
					errs = append(errs, fmt.Sprintf("%s: when references unknown variable %q", prefix, varName))
				}
				if pattern == "" {
					errs = append(errs, fmt.Sprintf("%s: when pattern for %q must not be empty", prefix, varName))
					continue
				}
				if _, err := regexp.Compile(fullMatchRegex(pattern)); err != nil {
					errs = append(errs, fmt.Sprintf("%s: invalid regex for %q: %v", prefix, varName, err))
				}
			}
		}
		if defaultCount > 1 {
			errs = append(errs, "at most one default branch allowed per command")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
