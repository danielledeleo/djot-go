package djot_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCase represents a single test from a .test file.
type TestCase struct {
	Name     string // description text before the test block
	Input    string
	Expected string
	File     string
	Line     int
	IsAST    bool // true if expected output is AST format (``` a blocks)
}

// parseTestFile reads a djot .test file and returns its test cases.
// Format: triple-backtick blocks with "." separating input from expected output.
// Text between blocks is used as the test name.
func parseTestFile(path string) ([]TestCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []TestCase
	var desc string
	var inBlock bool
	var inExpected bool
	var isAST bool
	var input, expected strings.Builder
	var blockLine int
	var blockFence string

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inBlock {
			if strings.HasPrefix(line, "```") {
				// Count the backtick fence length.
				fence := ""
				for _, c := range line {
					if c == '`' {
						fence += "`"
					} else {
						break
					}
				}
				blockFence = fence
				// Check for AST format marker (e.g., "``` a")
				suffix := strings.TrimSpace(line[len(fence):])
				isAST = suffix == "a"
				inBlock = true
				inExpected = false
				input.Reset()
				expected.Reset()
				blockLine = lineNum
			} else {
				// Accumulate description text.
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					desc = trimmed
				}
			}
			continue
		}

		// Inside a block.
		if strings.HasPrefix(line, blockFence) && strings.TrimSpace(line) == blockFence {
			// End of block — must match the exact opening fence.
			name := desc
			if name == "" {
				name = fmt.Sprintf("line %d", blockLine)
			}
			cases = append(cases, TestCase{
				Name:     name,
				Input:    input.String(),
				Expected: expected.String(),
				File:     filepath.Base(path),
				Line:     blockLine,
				IsAST:    isAST,
			})
			inBlock = false
			desc = ""
			continue
		}

		if !inExpected && line == "." {
			inExpected = true
			continue
		}

		if inExpected {
			if expected.Len() > 0 {
				expected.WriteByte('\n')
			}
			expected.WriteString(line)
		} else {
			if input.Len() > 0 {
				input.WriteByte('\n')
			}
			input.WriteString(line)
		}
	}

	return cases, scanner.Err()
}

// loadOfficialTests loads all .test files from testdata/official/,
// skipping files that need special handling.
func loadOfficialTests(t *testing.T) map[string][]TestCase {
	t.Helper()

	skip := map[string]bool{
		"filters.test":   true, // requires Lua filter execution
		"sourcepos.test": true, // AST output, not HTML
		"symb.test":      true, // AST output, not HTML
	}

	files, err := filepath.Glob("testdata/official/*.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test files found in testdata/official/")
	}

	result := make(map[string][]TestCase)
	for _, f := range files {
		base := filepath.Base(f)
		if skip[base] {
			continue
		}
		cases, err := parseTestFile(f)
		if err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}
		result[base] = cases
	}
	return result
}
