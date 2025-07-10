package main

import (
	"reflect"
	"testing"
)

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"running", "Running"},
		{"Colima is running", "Running"},
		{"Stopped", "Stopped"},
		{"  stopped  ", "Stopped"},
		{"", "Unknown"},
		{"something", "something"},
	}
	for _, c := range cases {
		got := parseStatus(c.in)
		if got != c.want {
			t.Errorf("parseStatus(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseContainerStatus(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Up 2 minutes", "Running"},
		{"running", "Running"},
		{"Exited (0) 2 hours ago", "Stopped"},
		{"created", "Stopped"},
		{"", "Unknown"},
		{"something", "something"},
	}
	for _, c := range cases {
		got := parseContainerStatus(c.in)
		if got != c.want {
			t.Errorf("parseContainerStatus(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseDockerPSOutput(t *testing.T) {
	out := "web|proj1|Up 1 minute\nredis|proj1|Exited (0)\ndb||Created\napi|proj2|Up 2 minutes\n"
	got := parseDockerPSOutput(out)
	want := map[string][]Container{
		"proj1": {
			{Name: "web", Project: "proj1", Status: "Up 1 minute"},
			{Name: "redis", Project: "proj1", Status: "Exited (0)"},
		},
		"default": {
			{Name: "db", Project: "default", Status: "Created"},
		},
		"proj2": {
			{Name: "api", Project: "proj2", Status: "Up 2 minutes"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDockerPSOutput mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
