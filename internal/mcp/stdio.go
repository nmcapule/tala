package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ServeStdio runs the MCP server over the standard MCP stdio framing.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readStdioMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		res, ok := s.processMessage(ctx, body)
		if !ok {
			continue
		}
		if err := writeStdioMessage(out, res); err != nil {
			return err
		}
	}
}

func readStdioMessage(reader *bufio.Reader) ([]byte, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(trimmed), "{") {
			return []byte(strings.TrimSpace(trimmed)), nil
		}

		contentLength, ok := parseContentLength(trimmed)
		for {
			headerLine, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			headerLine = strings.TrimRight(headerLine, "\r\n")
			if headerLine == "" {
				break
			}
			if value, found := parseContentLength(headerLine); found {
				contentLength = value
				ok = true
			}
		}
		if !ok || contentLength < 0 {
			return nil, fmt.Errorf("missing or invalid Content-Length header")
		}
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, err
		}
		return body, nil
	}
}

func parseContentLength(line string) (int, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok || !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return -1, true
	}
	return n, true
}

func writeStdioMessage(out io.Writer, res response) error {
	body, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = out.Write(body)
	return err
}
