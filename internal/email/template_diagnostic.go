package email

import (
	"errors"
	htmltemplate "html/template"
	"regexp"
	"strconv"
	"strings"
)

type TemplatePart string

const (
	TemplatePartSubject TemplatePart = "subject"
	TemplatePartText    TemplatePart = "text"
	TemplatePartHTML    TemplatePart = "html"
)

type TemplateErrorLocation struct {
	Part TemplatePart
	Line int
}

type templateSourceError struct {
	location TemplateErrorLocation
	err      error
}

func (e *templateSourceError) Error() string {
	return e.err.Error()
}

func (e *templateSourceError) Unwrap() error {
	return e.err
}

type templateOffsetError struct {
	offset int
	err    error
}

func (e *templateOffsetError) Error() string {
	return e.err.Error()
}

func (e *templateOffsetError) Unwrap() error {
	return e.err
}

var templateParseLinePattern = regexp.MustCompile(`template: [^:\n]+:(\d+)(?::\d+)?:`)

func LocateTemplateError(err error) (TemplateErrorLocation, bool) {
	var sourceErr *templateSourceError
	if !errors.As(err, &sourceErr) {
		return TemplateErrorLocation{}, false
	}
	return sourceErr.location, true
}

func annotateTemplateSourceError(part TemplatePart, source string, err error) error {
	if err == nil {
		return nil
	}
	var sourceErr *templateSourceError
	if errors.As(err, &sourceErr) {
		return err
	}
	return &templateSourceError{
		location: TemplateErrorLocation{
			Part: part,
			Line: templateErrorLine(source, err),
		},
		err: err,
	}
}

func templateErrorAt(offset int, err error) error {
	return &templateOffsetError{offset: offset, err: err}
}

func templateErrorLine(source string, err error) int {
	var offsetErr *templateOffsetError
	if errors.As(err, &offsetErr) && offsetErr.offset >= 0 {
		return lineAtOffset(source, offsetErr.offset)
	}

	var htmlErr *htmltemplate.Error
	if errors.As(err, &htmlErr) {
		if htmlErr.Line > 0 {
			return htmlErr.Line
		}
		if line := inferredHTMLContextErrorLine(source, htmlErr.Description); line > 0 {
			return line
		}
		if htmlErr.Node != nil {
			return lineAtOffset(source, int(htmlErr.Node.Position())-1)
		}
	}

	match := templateParseLinePattern.FindStringSubmatch(err.Error())
	if len(match) == 2 {
		line, parseErr := strconv.Atoi(match[1])
		if parseErr == nil && line > 0 {
			return line
		}
	}
	return 0
}

func inferredHTMLContextErrorLine(source, description string) int {
	if separator := strings.LastIndex(description, ": "); separator >= 0 {
		quoted := strings.TrimSpace(description[separator+2:])
		fragment, err := strconv.Unquote(quoted)
		if err == nil && fragment != "" {
			if offset := strings.Index(source, fragment); offset >= 0 {
				for offset < len(source) && (source[offset] == ' ' || source[offset] == '\t' || source[offset] == '\r' || source[offset] == '\n') {
					offset++
				}
				return lineAtOffset(source, offset)
			}
		}
	}

	switch {
	case strings.Contains(description, "delimDoubleQuote"):
		if offset := strings.LastIndexByte(source, '"'); offset >= 0 {
			return lineAtOffset(source, offset)
		}
	case strings.Contains(description, "delimSingleQuote"):
		if offset := strings.LastIndexByte(source, '\''); offset >= 0 {
			return lineAtOffset(source, offset)
		}
	case strings.Contains(description, "stateCSS"):
		if offset := strings.LastIndex(strings.ToLower(source), "<style"); offset >= 0 {
			return lineAtOffset(source, offset)
		}
	}

	trimmed := strings.TrimRight(source, " \t\r\n")
	if trimmed == "" {
		return 0
	}
	return lineAtOffset(source, len(trimmed)-1)
}

func lineAtOffset(source string, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return strings.Count(source[:offset], "\n") + 1
}

func malformedDelimiterOffset(value string) int {
	opening := strings.Index(value, "{{")
	closing := strings.Index(value, "}}")
	switch {
	case opening < 0:
		return closing
	case closing < 0:
		return opening
	case opening < closing:
		return opening
	default:
		return closing
	}
}
