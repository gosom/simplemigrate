package sqlfluff

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/gosom/simplemigrate"
)

type sqlFluffWrapper struct {
	sqlFluffPath string
}

func New() (simplemigrate.QueryValidator, error) {
	cmd := exec.Command("which", "sqlfluff")

	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("failed to find sqlfluff: make sure it's installed (pip install sqlfluff)")
	}

	ans := sqlFluffWrapper{
		sqlFluffPath: strings.TrimSpace(string(output)),
	}

	return &ans, nil
}

func (s *sqlFluffWrapper) ValidateQuery(ctx context.Context, dialect, query string) error {
	args := []string{
		"lint",
		"-n",
		"--dialect", dialect,
		"--rules", "disable",
		"-p", "1",
		"-",
	}

	cmd := exec.CommandContext(ctx, s.sqlFluffPath, args...) //nolint:gosec // TODO can I do better?

	var stdin bytes.Buffer

	stdin.WriteString(query)

	cmd.Stdin = &stdin

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}

	return nil
}
