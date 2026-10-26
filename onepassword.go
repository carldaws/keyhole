package main

import (
	"context"
	"strings"

	"github.com/1password/onepassword-sdk-go"
)

type onePassword struct {
	account string

	// The SDK mixes up desktop app calls that overlap, so each call waits its turn.
	turn   chan struct{}
	client *onepassword.Client
}

func newOnePassword(account string) *onePassword {
	return &onePassword{account: account, turn: make(chan struct{}, 1)}
}

func (o *onePassword) Resolve(ctx context.Context, reference string) (string, error) {
	select {
	case o.turn <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-o.turn }()

	secret, err := o.resolve(ctx, reference)
	if clientLost(err) {
		o.client = nil
		secret, err = o.resolve(ctx, reference)
	}
	return secret, err
}

func (o *onePassword) resolve(ctx context.Context, reference string) (string, error) {
	if o.client == nil {
		client, err := onepassword.NewClient(context.WithoutCancel(ctx),
			onepassword.WithDesktopAppIntegration(o.account),
			onepassword.WithIntegrationInfo("keyhole", buildVersion()),
		)
		if err != nil {
			return "", err
		}
		o.client = client
	}
	return o.client.Secrets().Resolve(ctx, reference)
}

func clientLost(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "invalid client id")
}
