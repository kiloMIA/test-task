package main

import (
	"context"
)

type Getter interface {
	Get(ctx context.Context, address, key string) (string, error)
}

func Get(ctx context.Context, getter Getter, addresses []string, key string) (string, error) {
	return "", nil
}
