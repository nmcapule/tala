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

type stdioMessage struct {
	body   []byte
	framed bool
}

// ServeStdio runs the MCP server over the standard MCP stdio framing.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		msg, err := readStdioMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		res, ok := s.processMessage(ctx, msg.body)
		if !ok {
			continue
		}
		if err := writeStdioMessage(out, res, msg.framed); err != nil {
			return err
		}
	}
}

func readStdioMessage(reader *bufio.Reader) (stdioMessage, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return stdioMessage{}, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(trimmed), "{") {
			return stdioMessage{body: []byte(strings.TrimSpace(trimmed))}, nil
		}

		contentLength, ok := parseContentLength(trimmed)
		for {
			headerLine, err := reader.ReadString('\n')
			if err != nil {
				return stdioMessage{}, err
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
			return stdioMessage{}, fmt.Errorf("missing or invalid Content-Length header")
		}
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return stdioMessage{}, err
		}
		return stdioMessage{body: body, framed: true}, nil
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

func writeStdioMessage(out io.Writer, res response, framed bool) error {
	body, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if !framed {
		if _, err := out.Write(body); err != nil {
			return err
		}
		_, err = out.Write([]byte("\n"))
		return err
	}
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = out.Write(body)
	return err
}
