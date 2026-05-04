package compose

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Parser parses an AgentCompose file into a Document.
type Parser interface {
	Parse(io.Reader) (Document, error)
}

// LineParser implements the line-oriented stdlib-friendly compose grammar.
type LineParser struct{}

// NewParser returns the default LineParser.
func NewParser() LineParser { return LineParser{} }

// Parse reads the compose document from reader. Lines starting with '#' and
// blank lines are ignored.
func (LineParser) Parse(reader io.Reader) (Document, error) {
	document := Document{}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if err := parseLine(&document, line); err != nil {
			return Document{}, fmt.Errorf("AgentCompose:%d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return Document{}, err
	}
	return document, nil
}

func parseLine(document *Document, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}

	directive := strings.ToUpper(fields[0])
	args := fields[1:]

	switch directive {
	case "COMPOSE":
		if len(args) != 1 {
			return fmt.Errorf("COMPOSE expects exactly one argument")
		}
		document.Name = args[0]
	case "AGENT":
		service, err := parseAgentLine(args)
		if err != nil {
			return err
		}
		document.Services = append(document.Services, service)
	default:
		return fmt.Errorf("unknown directive %q", fields[0])
	}
	return nil
}

func parseAgentLine(args []string) (Service, error) {
	if len(args) < 2 {
		return Service{}, fmt.Errorf("AGENT expects <name> FILE=<path> [DEPENDS_ON=<csv>]")
	}
	service := Service{Name: args[0]}

	for _, pair := range args[1:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return Service{}, fmt.Errorf("AGENT option %q must use key=value", pair)
		}
		switch strings.ToUpper(key) {
		case "FILE":
			service.File = value
		case "DEPENDS_ON":
			if value == "" {
				continue
			}
			for _, dependency := range strings.Split(value, ",") {
				dependency = strings.TrimSpace(dependency)
				if dependency == "" {
					continue
				}
				service.DependsOn = append(service.DependsOn, dependency)
			}
		default:
			return Service{}, fmt.Errorf("unknown AGENT option %q", key)
		}
	}
	return service, nil
}

func stripComment(line string) string {
	for index, char := range line {
		if char == '#' {
			return line[:index]
		}
	}
	return line
}
