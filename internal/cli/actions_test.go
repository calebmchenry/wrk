package cli

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"wrk/internal/runner"
)

func TestRunArguments(t *testing.T) {
	child := []string{"./command-{id}", "--json", "--help", "--", "", " two words ", "a{id}{id}", "$(touch x);*|>"}
	r, err := Parse(append([]string{"--project", "../other", "run", "--ready", "--label=a", "--label=b", "--under=wrk-12345678", "--expect-status=done", "--max-tickets=2", "--"}, child...))
	if err != nil || !reflect.DeepEqual(r.ChildCommand, child) || r.JSON || r.Help || !r.Ready || r.MaxTickets != 2 || *r.ExpectStatus != "done" || *r.Selection.Project != "../other" || len(r.Labels) != 2 {
		t.Fatalf("request %+v: %v", r, err)
	}
	for _, args := range [][]string{
		{"run"}, {"run", "--"}, {"run", "--", ""}, {"run", "--", "  "},
		{"run", "echo"}, {"run", "echo", "--", "id"},
		{"run", "--all", "--ready", "--", "true"},
		{"run", "--ticket=wrk-12345678", "--ready", "--", "true"},
		{"run", "--ticket=wrk-12345678", "--all", "--", "true"},
		{"run", "--ticket=wrk-12345678", "--label=x", "--", "true"},
		{"run", "--ticket=wrk-12345678", "--under=wrk-abcdefab", "--", "true"},
		{"run", "--ticket=", "--", "true"},
		{"run", "--ticket=bad", "--", "true"},
		{"run", "--expect-status=", "--", "true"},
		{"run", "--expect-status=closed", "--", "true"},
		{"run", "--max-tickets=0", "--", "true"},
		{"run", "--max-tickets=-1", "--", "true"},
		{"run", "--max-tickets=1.5", "--", "true"},
		{"run", "--max-tickets=999999999999999999999999", "--", "true"},
		{"run", "--max-tickets=1", "--max-tickets=2", "--", "true"},
		{"run", "--label=", "--", "true"},
		{"run", "--ticket=wrk-12345678", "--stream", "--", "true"},
		{"run", "--poll-interval=1s", "--", "true"},
		{"run", "--stream=false", "--", "true"},
		{"run", "--stream", "--stream", "--", "true"},
		{"run", "--stream", "--poll-interval=1s", "--poll-interval=2s", "--", "true"},
		{"list", "--stream"}, {"list", "--poll-interval=1s"},
		{"list", "--expect-status=done"}, {"show", "--ticket=wrk-12345678"},
	} {
		if _, err := Parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	for _, args := range [][]string{
		{"run", "--help"}, {"help", "run"},
		{"run", "--ticket=wrk-12345678", "--max-tickets=2", "--", "true"},
		{"run", "--all", "--expect-status=blocked", "--", "true"},
		{"run", "--config=../other/.wrk/config.yaml", "--", "true"},
	} {
		if _, err := Parse(args); err != nil {
			t.Errorf("rejected %q: %v", args, err)
		}
	}
	if JSONRequested([]string{"run", "--", "cmd", "--json"}) {
		t.Fatal("child flag selected runner JSON")
	}
	for _, flag := range []string{"--ticket", "--max-tickets", "--expect-status", "--stream", "--poll-interval", "{id}"} {
		if !strings.Contains(Usage, flag) {
			t.Errorf("help missing %s", flag)
		}
	}
}

func TestStreamArguments(t *testing.T) {
	for _, interval := range []string{"", "0", "0s", "-1s", "1", "soon", "1d", "999999999999999999h", "0.1ns"} {
		if _, err := Parse([]string{"run", "--stream", "--poll-interval=" + interval, "--", "true"}); err == nil {
			t.Errorf("accepted interval %q", interval)
		}
	}
	for _, interval := range []string{"1ns", "250ms", "1.5s", "2m"} {
		r, err := Parse([]string{"run", "--poll-interval", interval, "--stream", "--ready", "--max-tickets=2", "--", "true", "--stream", "--poll-interval=bad"})
		want, _ := time.ParseDuration(interval)
		if err != nil || !r.Stream || r.PollInterval != want || len(r.ChildCommand) != 3 {
			t.Fatalf("interval %q: %+v %v", interval, r, err)
		}
	}
	for _, args := range [][]string{{"run", "--stream", "--", "true"}, {"run", "--", "true"}} {
		r, err := Parse(args)
		if err != nil || r.PollInterval != runner.DefaultPollInterval {
			t.Fatalf("default: %+v %v", r, err)
		}
	}
}
