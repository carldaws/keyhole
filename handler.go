package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
)

const maxRequestBytes = 1 << 20

type resolver interface {
	Resolve(ctx context.Context, reference string) (string, error)
}

type errorReply struct {
	Error string `json:"error"`
}

func newHandler(secrets resolver, path, wrap string) http.Handler {
	pattern := "POST " + path
	if strings.HasSuffix(path, "/") {
		pattern += "{$}"
	}
	mux := http.NewServeMux()
	mux.Handle(pattern, resolveAll(secrets, wrap))
	return mux
}

func resolveAll(secrets resolver, wrap string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		references, err := decodeReferences(http.MaxBytesReader(w, r.Body, maxRequestBytes), wrap)
		if err != nil {
			reply(w, http.StatusBadRequest, errorReply{err.Error()})
			return
		}

		values := make(map[string]string, len(references))
		var failures []string
		for _, name := range slices.Sorted(maps.Keys(references)) {
			reference := references[name]
			value, err := secrets.Resolve(r.Context(), reference)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s (%s): %v", name, reference, err))
				continue
			}
			values[name] = value
		}
		if len(failures) > 0 {
			reply(w, http.StatusBadGateway, errorReply{"could not resolve " + strings.Join(failures, "; ")})
			return
		}
		reply(w, http.StatusOK, values)
	}
}

func decodeReferences(body io.Reader, wrap string) (map[string]string, error) {
	decoder := json.NewDecoder(body)
	var references map[string]string
	if wrap == "" {
		if err := decoder.Decode(&references); err != nil {
			return nil, err
		}
	} else {
		var wrapper map[string]json.RawMessage
		if err := decoder.Decode(&wrapper); err != nil {
			return nil, err
		}
		inner, ok := wrapper[wrap]
		if !ok || len(wrapper) != 1 {
			return nil, fmt.Errorf("request must hold %q and nothing else", wrap)
		}
		if err := json.Unmarshal(inner, &references); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("request has more than one JSON value")
	}
	if references == nil {
		return nil, errors.New("request has no references")
	}
	return references, nil
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
