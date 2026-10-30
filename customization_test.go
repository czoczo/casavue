package main

import "testing"

func TestRegexpReplaceStringKeepsReplacementLiteral(t *testing.T) {
	s := "<title>Old Title</title>"
	regexpReplaceString(&s, "<title>", "[^<]*", `Home $1 $HOME`, "</title>")
	if s != "<title>Home $1 $HOME</title>" {
		t.Errorf("got %q", s)
	}
}
