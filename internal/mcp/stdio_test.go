package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

func TestMCPStdioTransportHandlesFramedRequests(t *testing.T) {
	server := newTestServer(t)
	var input bytes.Buffer
	writeTestFrame(t, &input, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"tala-test","version":"0.0.0"},"capabilities":{}}}`)
	writeTestFrame(t, &input, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	writeTestFrame(t, &input, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	writeTestFrame(t, &input, `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"tala://planning"}}`)

	var output bytes.Buffer
	if err := server.ServeStdio(context.Background(), &input, &output); err != nil {
		t.Fatal(err)
	}

	responses := decodeTestFrames(t, output.Bytes())
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	assertResponseID(t, responses[0], float64(1))
	initialize := responses[0]["result"].(map[string]any)
	serverInfo := initialize["serverInfo"].(map[string]any)
	if serverInfo["name"] != "tala" {
		t.Fatalf("unexpected initialize serverInfo: %#v", serverInfo)
	}

	assertResponseID(t, responses[1], float64(2))
	tools := responses[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 12 {
		t.Fatalf("expected 12 tools, got %d", len(tools))
	}

	assertResponseID(t, responses[2], float64(3))
	contents := responses[2]["result"].(map[string]any)["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("expected one planning resource content item, got %#v", contents)
	}
}

func TestMCPStdioTransportHandlesJSONLinesAndInvalidRequests(t *testing.T) {
	server := newTestServer(t)
	input := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\" \n")

	var output bytes.Buffer
	if err := server.ServeStdio(context.Background(), input, &output); err != nil {
		t.Fatal(err)
	}

	responses := decodeTestLines(t, output.Bytes())
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if responses[0]["error"] != nil {
		t.Fatalf("expected first JSONL request to succeed, got %#v", responses[0]["error"])
	}
	rpcError := responses[1]["error"].(map[string]any)
	if rpcError["code"] != float64(-32700) {
		t.Fatalf("expected parse error for invalid JSONL request, got %#v", rpcError)
	}
}

func writeTestFrame(t *testing.T, buf *bytes.Buffer, body string) {
	t.Helper()
	if _, err := fmt.Fprintf(buf, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		t.Fatal(err)
	}
}

func decodeTestFrames(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	reader := bufio.NewReader(bytes.NewReader(data))
	var responses []map[string]any
	for {
		if _, err := reader.Peek(1); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		msg, err := readStdioMessage(reader)
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(msg.body, &response); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	return responses
}

func decodeTestLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var responses []map[string]any
	for scanner.Scan() {
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return responses
}

func assertResponseID(t *testing.T, response map[string]any, want any) {
	t.Helper()
	if response["id"] != want {
		t.Fatalf("expected response id %#v, got %#v in %#v", want, response["id"], response)
	}
}
