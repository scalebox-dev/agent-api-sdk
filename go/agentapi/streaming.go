package agentapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type ResponseStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	event   ResponseStreamEvent
	err     error
}

func newResponseStream(resp *http.Response) *ResponseStream {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &ResponseStream{resp: resp, scanner: scanner}
}

func (s *ResponseStream) Next() bool {
	if s == nil || s.err != nil {
		return false
	}
	var block bytes.Buffer
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if strings.TrimSpace(line) == "" {
			if block.Len() == 0 {
				continue
			}
			return s.parseBlock(block.String())
		}
		block.WriteString(line)
		block.WriteByte('\n')
	}
	if err := s.scanner.Err(); err != nil {
		s.err = err
		return false
	}
	if block.Len() > 0 {
		return s.parseBlock(block.String())
	}
	return false
}

func (s *ResponseStream) Event() ResponseStreamEvent { return s.event }

func (s *ResponseStream) Err() error { return s.err }

func (s *ResponseStream) Close() error {
	if s == nil || s.resp == nil || s.resp.Body == nil {
		return nil
	}
	return s.resp.Body.Close()
}

func (s *ResponseStream) parseBlock(block string) bool {
	data := sseData(block)
	if data == "" || data == "[DONE]" {
		return s.Next()
	}
	var ev ResponseStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		s.err = err
		return false
	}
	ev.Raw = append([]byte(nil), data...)
	s.event = ev
	return true
}

func sseData(block string) string {
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimLeft(line[len("data:"):], " \t"))
		}
	}
	return strings.Join(lines, "\n")
}

func readAllAndClose(r io.ReadCloser) ([]byte, error) {
	defer r.Close()
	return io.ReadAll(r)
}
