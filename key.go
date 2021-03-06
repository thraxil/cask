package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type key struct {
	Algorithm string
	Value     []byte
}

func keyFromPath(path string) (*key, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 21 {
		return nil, errors.New("not enough parts")
	}
	algorithm := parts[len(parts)-21]
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return nil, fmt.Errorf("invalid hash length: %d (%s)", len(hash), hash)
	}
	return keyFromString(algorithm + ":" + hash)
}

func keyFromString(str string) (*key, error) {
	parts := strings.Split(str, ":")
	algorithm := parts[0]
	if algorithm != "sha1" {
		return nil, errors.New("can only handle sha1 now")
	}
	str = parts[1]
	if len(str) != 40 {
		return nil, errors.New("invalid key")
	}
	return &key{algorithm, []byte(str)}, nil
}

func (k key) AsPath() string {
	var parts []string
	s := string(k.Value)
	for i := range s {
		if (i % 2) != 0 {
			parts = append(parts, s[i-1:i+1])
		}
	}
	return strings.Join(parts, "/")
}

func (k key) String() string {
	return k.Algorithm + ":" + string(k.Value)
}

func (k key) Valid() bool {
	// at the moment only sha1 is supported
	return k.Algorithm == "sha1" && len(k.Value) == 40
}
