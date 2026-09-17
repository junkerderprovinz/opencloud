package main

import "testing"

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:9299":    true,
		"[::1]:9299":        true,
		"localhost:9200":    true,
		"127.0.0.1":         true,
		"0.0.0.0:9299":      false,
		"192.168.1.10:9200": false,
		"cloud.example.com": false,
		":9299":             false,
	}
	for in, want := range cases {
		if got := isLoopback(in); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", in, got, want)
		}
	}
}
