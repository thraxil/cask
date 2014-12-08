package main

type Backend interface {
	Write(key string, data []byte)
	Read(key string) ([]byte, error)
	Exists(key string) bool
}

type DiskBackend struct {
	Root string
}

func NewDiskBackend(root string) *DiskBackend {
	return &DiskBackend{Root: root}
}

func (d *DiskBackend) Write(key string, data []byte) {

}

func (d DiskBackend) Read(key string) ([]byte, error) {
	return []byte(""), nil
}

func (d DiskBackend) Exists(key string) bool {
	return true
}
