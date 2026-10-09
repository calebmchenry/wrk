package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"wrk/internal/store"
)

func TestServeArguments(t *testing.T) {
	for _, args := range [][]string{{"serve"}, {"serve", "--port=0", "--open"}, {"--project=path", "serve", "--port", "65535"}, {"serve", "--config=path/.wrk/config.yaml", "--json"}} {
		r, err := Parse(args)
		if err != nil {
			t.Fatal(args, err)
		}
		if len(args) == 1 && r.Port != 7331 {
			t.Fatal(r)
		}
	}
	for _, args := range [][]string{
		{"serve", "--port=-1"}, {"serve", "--port=65536"}, {"serve", "--port="}, {"serve", "--port=1.5"},
		{"serve", "--port=abc"}, {"serve", "--port= 80"}, {"serve", "--port"}, {"serve", "--port=80", "--port=81"},
		{"serve", "--open", "--open"}, {"serve", "--open=true"}, {"serve", "--host=0.0.0.0"}, {"serve", "extra"},
		{"serve", "--project=a", "--config=b"}, {"list", "--port=0"},
	} {
		if _, err := Parse(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
	var out, errout bytes.Buffer
	if code := Run([]string{"serve", "--project=missing", "--help", "--json"}, t.TempDir(), nil, &out, &errout); code != 0 {
		t.Fatal(code, out.String(), errout.String())
	}
}

func TestBrowserFailureDoesNotStopServer(t *testing.T) {
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	var stderr bytes.Buffer
	ended := make(chan int, 1)
	opened := false
	go func() {
		ended <- serve(ctx, Request{Command: "serve", JSON: true, Open: true, Port: 0}, root, outWriter, &stderr, func(_ context.Context, url string) error {
			// The URL is usable before trying the browser, even if launching fails.
			client := &http.Client{Timeout: time.Second}
			response, err := client.Get(url + "api/project")
			if err != nil {
				return err
			}
			response.Body.Close()
			opened = response.StatusCode == http.StatusOK
			return errors.New("no browser installed")
		})
		outWriter.Close()
	}()
	dec := json.NewDecoder(outReader)
	for _, event := range []string{"started", "warning"} {
		var e Envelope
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		m := e.Result.(map[string]any)
		if !e.OK || len(e.Errors) > 0 || m["event"] != event || *e.ProjectRoot != root {
			t.Fatal(e)
		}
	}
	select {
	case code := <-ended:
		t.Fatal("browser failure stopped server", code)
	default:
	}
	cancel()
	var e Envelope
	if err := dec.Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Result.(map[string]any)["event"] != "stopped" {
		t.Fatal(e)
	}
	select {
	case code := <-ended:
		if code != 0 {
			t.Fatal(code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
	if !opened || stderr.Len() != 0 {
		t.Fatal(opened, stderr.String())
	}
	if dec.Decode(new(any)) != io.EOF {
		t.Fatal("extra lifecycle output")
	}
}
