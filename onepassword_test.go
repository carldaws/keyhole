package main

import (
	"context"
	"errors"
	"testing"
)

func TestClientLost(t *testing.T) {
	cases := []struct {
		name string
		err  error
		lost bool
	}{
		{"the app forgot our client without saying the session expired", errors.New("an internal error occurred: Invalid client ID"), true},
		{"the reference is wrong", errors.New("no item matched the secret reference query"), false},
		{"nothing went wrong", nil, false},
	}
	for _, c := range cases {
		if got := clientLost(c.err); got != c.lost {
			t.Errorf("%s: clientLost(%v) = %v", c.name, c.err, got)
		}
	}
}

func TestResolveGivesUpWhileWaitingItsTurn(t *testing.T) {
	o := newOnePassword("Home")
	o.turn <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := o.Resolve(ctx, "op://Home/Postgres/password"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
