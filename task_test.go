package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type Response struct {
	Value string
	Error error
	Delay time.Duration
}

type MockGetter struct {
	Responses map[string]map[string]Response
}

func NewMockGetter(responses map[string]map[string]Response) *MockGetter {
	return &MockGetter{
		Responses: responses,
	}
}

func (m *MockGetter) Get(ctx context.Context, address, key string) (string, error) {
	if responses, exists := m.Responses[address]; exists {
		if resp, keyExists := responses[key]; keyExists {
			if resp.Delay > 0 {
				select {
				case <-time.After(resp.Delay):
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}

			if resp.Error != nil {
				return "", resp.Error
			}

			return resp.Value, nil
		}
	}

	return "", errors.New("key not found")
}

func TestGet(t *testing.T) {
	tests := []struct {
		name      string
		responses map[string]map[string]Response
		addresses []string
		key       string
		ttl       time.Duration
		wantValue string
		wantErr   bool
		wantErrIs error
	}{
		{
			name: "первый адрес падает, второй успешен",
			responses: map[string]map[string]Response{
				"addr1": {"key1": {Error: errors.New("connection error")}},
				"addr2": {"key1": {Value: "value2"}},
			},
			addresses: []string{"addr1", "addr2"},
			key:       "key1",
			ttl:       50 * time.Millisecond,
			wantValue: "value2",
			wantErr:   false,
		},
		{
			name: "все адреса падают",
			responses: map[string]map[string]Response{
				"addr1": {"key1": {Error: errors.New("error 1")}},
				"addr2": {"key1": {Error: errors.New("error 2")}},
			},
			addresses: []string{"addr1", "addr2"},
			key:       "key1",
			ttl:       50 * time.Millisecond,
			wantValue: "",
			wantErr:   true,
		},
		{
			name: "отмена контекста",
			responses: map[string]map[string]Response{
				"addr1": {"key1": {Value: "value1", Delay: 200 * time.Millisecond}},
			},
			addresses: []string{"addr1"},
			key:       "key1",
			ttl:       50 * time.Millisecond,
			wantValue: "",
			wantErr:   true,
			wantErrIs: context.Canceled,
		},
		{
			name: "быстрый адрес побеждает медленный",
			responses: map[string]map[string]Response{
				"addr1": {"key1": {Value: "value1", Delay: 200 * time.Millisecond}},
				"addr2": {"key1": {Value: "value2", Delay: 50 * time.Millisecond}},
			},
			addresses: []string{"addr1", "addr2"},
			key:       "key1",
			ttl:       300 * time.Millisecond,
			wantValue: "value2",
			wantErr:   false,
		},
		{
			name:      "пустой список адресов",
			responses: map[string]map[string]Response{},
			addresses: []string{},
			key:       "key1",
			ttl:       50 * time.Millisecond,
			wantValue: "",
			wantErr:   false,
		},

		{
			name: "смешанные ошибки: key not found, connection error и один успех",
			responses: map[string]map[string]Response{
				"addr1": {},
				"addr2": {"key1": {Error: errors.New("connection error")}},
				"addr3": {"key1": {Value: "value3"}},
			},
			addresses: []string{"addr1", "addr2", "addr3"},
			key:       "key1",
			ttl:       200 * time.Millisecond,
			wantValue: "value3",
			wantErr:   false,
		},
		{
			name: "один адрес, ключ отсутствует",
			responses: map[string]map[string]Response{
				"addr1": {},
			},
			addresses: []string{"addr1"},
			key:       "key1",
			ttl:       50 * time.Millisecond,
			wantValue: "",
			wantErr:   true,
		},
		{
			name: "дубликаты адресов: первые два падают, третий успешен",
			responses: map[string]map[string]Response{
				"addr1": {"key1": {Error: errors.New("err")}},
				"addr2": {"key1": {Value: "value2"}},
			},
			addresses: []string{"addr1", "addr1", "addr2"},
			key:       "key1",
			ttl:       100 * time.Millisecond,
			wantValue: "value2",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockGetter(tt.responses)

			var getter Getter = mock

			ctx, cancel := context.WithTimeout(context.Background(), tt.ttl)
			defer cancel()

			got, err := Get(ctx, getter, tt.addresses, tt.key)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Fatalf("Get() error = %v, want errors.Is(err, %v) == true", err, tt.wantErrIs)
			}

			if got != tt.wantValue {
				t.Fatalf("Get() = %q, want %q", got, tt.wantValue)
			}
		})
	}
}
