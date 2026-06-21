// genschema parses migrations/*.up.sql, builds a Mermaid ER diagram, and
// patches it into README.md between <!-- schema:start --> and <!-- schema:end -->.
//
// Run from the project root:
//
//	go run ./scripts/genschema
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	migrationsDir = "migrations"
	readmePath    = "README.md"
	startMarker   = "<!-- schema:start -->"
	endMarker     = "<!-- schema:end -->"
)

type column struct {
	name string
	typ  string
	pk   bool
	fk   bool
	uk   bool
}

type foreignKey struct {
	srcCol   string
	dstTable string
}

type table struct {
	name string
	cols []column
	fks  []foreignKey
}

var (
	reCreateTable = regexp.MustCompile(`(?i)CREATE TABLE IF NOT EXISTS (\w+)`)
	rePK          = regexp.MustCompile(`(?i)PRIMARY KEY\s*\(([^)]+)\)`)
	reUK          = regexp.MustCompile(`(?i)UNIQUE KEY \w+ \((\w+)\)`)
	reFK          = regexp.MustCompile(`(?i)FOREIGN KEY\s*\((\w+)\)\s*REFERENCES\s+(\w+)`)

	// Lines starting with these (lowercased) are not column definitions.
	skipPrefixes = []string{
		"primary key", "unique key", "constraint", "key ", "index ", ") engine", ")",
	}
	skipKeywords = map[string]bool{
		"primary": true, "unique": true, "constraint": true,
		"key": true, "index": true,
	}
)

func main() {
	tables, err := parseMigrations(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genschema: parse migrations: %v\n", err)
		os.Exit(1)
	}
	diagram := mermaidERD(tables)
	if err := patchReadme(readmePath, diagram); err != nil {
		fmt.Fprintf(os.Stderr, "genschema: patch readme: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("README.md — Mermaid ER diagram updated.")
}

// parseMigrations reads all *.up.sql files in dir and returns all CREATE TABLE definitions.
func parseMigrations(dir string) ([]table, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var out []table
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // paths come from filepath.Glob on a known directory
		if err != nil {
			return nil, err
		}
		out = append(out, parseSQL(string(data))...)
	}
	return out, nil
}

// parseSQL extracts table definitions from a SQL string.
func parseSQL(sql string) []table {
	var tables []table
	var cur *table
	inTable := false

	for _, raw := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(raw)
		lower := strings.ToLower(trimmed)

		if m := reCreateTable.FindStringSubmatch(trimmed); m != nil {
			inTable = true
			cur = &table{name: m[1]}
			continue
		}
		if !inTable || cur == nil {
			continue
		}
		if strings.HasPrefix(lower, ")") {
			tables = append(tables, *cur)
			cur = nil
			inTable = false
			continue
		}
		processTableLine(cur, trimmed, lower)
	}
	return tables
}

// processTableLine handles a single line inside a CREATE TABLE block.
func processTableLine(cur *table, trimmed, lower string) {
	if m := rePK.FindStringSubmatch(trimmed); m != nil {
		markPKColumns(cur, m[1])
		return
	}
	if m := reUK.FindStringSubmatch(trimmed); m != nil {
		markUKColumn(cur, strings.TrimSpace(m[1]))
		return
	}
	if m := reFK.FindStringSubmatch(trimmed); m != nil {
		addFK(cur, strings.TrimSpace(m[1]), strings.TrimSpace(m[2]))
		return
	}
	if !shouldSkip(lower) {
		addColumnIfValid(cur, trimmed)
	}
}

func markPKColumns(cur *table, colList string) {
	for _, c := range strings.Split(colList, ",") {
		name := strings.TrimSpace(c)
		for i := range cur.cols {
			if cur.cols[i].name == name {
				cur.cols[i].pk = true
			}
		}
	}
}

func markUKColumn(cur *table, name string) {
	for i := range cur.cols {
		if cur.cols[i].name == name {
			cur.cols[i].uk = true
		}
	}
}

func addFK(cur *table, src, dst string) {
	cur.fks = append(cur.fks, foreignKey{srcCol: src, dstTable: dst})
	for i := range cur.cols {
		if cur.cols[i].name == src {
			cur.cols[i].fk = true
		}
	}
}

func addColumnIfValid(cur *table, trimmed string) {
	fields := strings.Fields(trimmed)
	if len(fields) < 2 || skipKeywords[strings.ToLower(fields[0])] {
		return
	}
	cur.cols = append(cur.cols, column{
		name: fields[0],
		typ:  normalizeType(fields[1]),
	})
}

func shouldSkip(lower string) bool {
	for _, p := range skipPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return lower == "" || strings.HasPrefix(lower, "--")
}

// normalizeType returns just the base SQL type in lowercase (e.g. "varchar", "bigint").
func normalizeType(s string) string {
	t := strings.ToLower(s)
	if idx := strings.Index(t, "("); idx > 0 {
		t = t[:idx]
	}
	return strings.Fields(t)[0]
}

// mermaidERD generates the Mermaid ERD block for the given tables.
func mermaidERD(tables []table) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\nerDiagram\n")

	for _, t := range tables {
		fmt.Fprintf(&sb, "    %s {\n", t.name)
		for _, c := range t.cols {
			attr := ""
			switch {
			case c.pk && c.fk:
				attr = " PK,FK"
			case c.pk:
				attr = " PK"
			case c.fk:
				attr = " FK"
			case c.uk:
				attr = " UK"
			}
			fmt.Fprintf(&sb, "        %s %s%s\n", c.typ, c.name, attr)
		}
		sb.WriteString("    }\n")
	}

	for _, t := range tables {
		for _, fk := range t.fks {
			fmt.Fprintf(&sb, "    %s ||--o{ %s : %q\n", fk.dstTable, t.name, fk.srcCol)
		}
	}

	sb.WriteString("```")
	return sb.String()
}

// patchReadme replaces the content between startMarker and endMarker in the file.
func patchReadme(path, content string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is the README constant, not user input
	if err != nil {
		return err
	}
	src := string(data)

	si := strings.Index(src, startMarker)
	ei := strings.Index(src, endMarker)
	if si == -1 || ei == -1 || si >= ei {
		return fmt.Errorf("markers %q / %q not found in %s", startMarker, endMarker, path)
	}

	patched := src[:si+len(startMarker)] + "\n" + content + "\n" + src[ei:]
	return os.WriteFile(path, []byte(patched), 0o600) //nolint:gosec // README is not sensitive; 0600 satisfies G306
}
