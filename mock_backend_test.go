package main

import (
	"io"
)

type MockBackend struct {
	freeSpace uint64
}

func (m MockBackend) String() string                        { return "mock" }
func (m MockBackend) Write(key key, r io.ReadCloser) error  { return nil }
func (m MockBackend) Read(key key) ([]byte, error)          { return nil, nil }
func (m MockBackend) Exists(key key) bool                   { return false }
func (m MockBackend) Delete(key key) error                  { return nil }
func (m MockBackend) ActiveAntiEntropy(c *cluster, s site, i int) {}
func (m MockBackend) NewVerifier(c *cluster) verifier       { return &MockVerifier{} }
func (m MockBackend) FreeSpace() uint64                     { return m.freeSpace }

type MockBackendFull struct {
	MockBackend
	data       map[string][]byte
	deletedKey string
	exists     bool
}

func (m *MockBackendFull) Read(k key) ([]byte, error) {
	if d, ok := m.data[k.String()]; ok {
		return d, nil
	}
	return nil, io.EOF
}

func (m *MockBackendFull) Delete(k key) error {
	m.deletedKey = k.String()
	return nil
}

func (m *MockBackendFull) Exists(k key) bool {
	if m.exists {
		return true
	}
	_, ok := m.data[k.String()]
	return ok
}

func (m *MockBackendFull) Write(k key, r io.ReadCloser) error {
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.data[k.String()] = b
	return nil
}

func (m *MockBackendFull) FreeSpace() uint64 { return 1000 }

type MockVerifier struct{}

func (m *MockVerifier) Verify(path string, k key, h string) error { return nil }
func (m *MockVerifier) VerifyKey(k key) error                  { return nil }
