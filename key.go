package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type Key struct {
	Algorithm string
	Value     []byte
}

func KeyFromPath(path string) (*Key, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return nil, errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return nil, errors.New(fmt.Sprintf("invalid hash length: %d (%s)", len(hash), hash))
	}
	return KeyFromString("sha1:" + hash)
}

func KeyFromString(str string) (*Key, error) {
	parts := strings.Split(str, ":")
	algorithm := parts[0]
	if algorithm != "sha1" {
		return nil, errors.New("can only handle sha1 now")
	}
	str = parts[1]
	if len(str) != 40 {
		return nil, errors.New("invalid key")
	}
	return &Key{algorithm, []byte(str)}, nil
}

func (k Key) AsPath() string {
	var parts []string
	s := k.String()
	for i := range s {
		if (i % 2) != 0 {
			parts = append(parts, s[i-1:i+1])
		}
	}
	return strings.Join(parts, "/")
}

func (k Key) String() string {
	return k.Algorithm + ":" + string(k.Value)
}

func (k Key) Valid() bool {
	return k.Algorithm == "sha1" && len(k.String()) == 40
}
