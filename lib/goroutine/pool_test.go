package goroutine

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/djylb/nps/lib/file"
	"github.com/panjf2000/ants/v2"
)

type chunkReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	n := copy(p, chunk)
	return n, nil
}

func TestCopyBuffer_HTTPDetectionAndCopy(t *testing.T) {
	input := "GET /hello HTTP/1.1\r\nHost: example.com\r\n\r\n"
	src := bytes.NewBufferString(input)
	var dst bytes.Buffer

	task := &file.Tunnel{Target: &file.Target{TargetStr: "127.0.0.1:80"}}
	written, err := CopyBuffer(&dst, src, nil, task, "127.0.0.1:12345")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != int64(len(input)) {
		t.Fatalf("unexpected written bytes, got=%d want=%d", written, len(input))
	}
	if dst.String() != input {
		t.Fatalf("unexpected copied content, got=%q want=%q", dst.String(), input)
	}
	if !task.IsHttp {
		t.Fatalf("expected task.IsHttp=true for HTTP request")
	}
}

func TestCopyBuffer_HTTPDetectionWithoutTargetDoesNotPanic(t *testing.T) {
	input := "GET /hello HTTP/1.1\r\nHost: example.com\r\n\r\n"
	src := bytes.NewBufferString(input)
	var dst bytes.Buffer

	task := &file.Tunnel{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CopyBuffer() panicked with nil target: %v", r)
		}
	}()

	written, err := CopyBuffer(&dst, src, nil, task, "127.0.0.1:12345")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != int64(len(input)) {
		t.Fatalf("unexpected written bytes, got=%d want=%d", written, len(input))
	}
	if !task.IsHttp {
		t.Fatal("expected task.IsHttp=true for HTTP request without target metadata")
	}
}

func TestCopyBuffer_DetectsFragmentedHTTPMethod(t *testing.T) {
	src := &chunkReader{
		chunks: [][]byte{
			[]byte("GE"),
			[]byte("T /hello HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		},
	}
	var dst bytes.Buffer

	task := &file.Tunnel{}
	written, err := CopyBuffer(&dst, src, nil, task, "127.0.0.1:12345")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != int64(dst.Len()) {
		t.Fatalf("unexpected written bytes, got=%d want=%d", written, dst.Len())
	}
	if !task.IsHttp {
		t.Fatal("expected fragmented HTTP method to be detected")
	}
}

func TestCopyBuffer_NonHTTPPayloadDoesNotClearDetectedHTTP(t *testing.T) {
	src := bytes.NewBufferString("SSH-2.0-test\r\n")
	var dst bytes.Buffer

	task := &file.Tunnel{IsHttp: true}
	written, err := CopyBuffer(&dst, src, nil, task, "127.0.0.1:12345")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != int64(dst.Len()) {
		t.Fatalf("unexpected written bytes, got=%d want=%d", written, dst.Len())
	}
	if !task.IsHttp {
		t.Fatal("non-HTTP payload should not clear previously detected HTTP capability")
	}
}

func TestCopyBufferKeepsCopyingPastLegacyFlowLimit(t *testing.T) {
	src := bytes.NewBufferString("abcd")
	var dst bytes.Buffer

	flow := &file.Flow{FlowLimit: 1, ExportFlow: (1 << 20) - 2}
	written, err := CopyBuffer(&dst, src, []*file.Flow{flow}, nil, "")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != 4 {
		t.Fatalf("unexpected written bytes, got=%d want=4", written)
	}
	if flow.InletFlow != 4 || flow.ExportFlow != (1<<20)+2 {
		t.Fatalf("unexpected flow snapshot inlet=%d export=%d", flow.InletFlow, flow.ExportFlow)
	}
}

func TestCopyBufferIgnoresLegacyTimeLimitChecks(t *testing.T) {
	src := bytes.NewBufferString("abc")
	var dst bytes.Buffer

	flow := &file.Flow{TimeLimit: time.Now().Add(-time.Second)}
	written, err := CopyBuffer(&dst, src, []*file.Flow{flow}, nil, "")
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if written != 3 {
		t.Fatalf("unexpected written bytes, got=%d want=3", written)
	}
	if flow.InletFlow != 3 || flow.ExportFlow != 3 {
		t.Fatalf("unexpected flow snapshot inlet=%d export=%d", flow.InletFlow, flow.ExportFlow)
	}
}

func TestCopyConnsHandlesConnCopyPoolInvokeFailure(t *testing.T) {
	oldPool := connCopyPool
	failingPool, err := ants.NewPoolWithFunc(1, func(interface{}) {}, ants.WithNonblocking(false))
	if err != nil {
		t.Fatalf("NewPoolWithFunc() error = %v", err)
	}
	failingPool.Release()
	connCopyPool = failingPool
	t.Cleanup(func() {
		connCopyPool = oldPool
	})

	left, right := net.Pipe()
	defer func() { _ = left.Close() }()
	defer func() { _ = right.Close() }()

	var outerWG sync.WaitGroup
	outerWG.Add(1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		copyConns(NewConns(left, right, nil, &outerWG, nil))
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("copyConns() hung after connCopyPool invoke failure")
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		outerWG.Wait()
	}()

	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("outer wait group was not released after connCopyPool invoke failure")
	}
}
