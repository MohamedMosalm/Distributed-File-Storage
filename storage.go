package main

import (
	// "bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const DefaultRootDirNAme = "root"

func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from := i * blocksize
		to := from + blocksize
		paths[i] = hashStr[from:to]
	}

	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

type PathTransformFunc func(string) PathKey

type PathKey struct {
	PathName string
	Filename string
}

func (k PathKey) FullPath(root string) string {
	return fmt.Sprintf("%s/%s/%s", root, k.PathName, k.Filename)
}

var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}

type StoreOpts struct {
	Root              string
	PathTransformFunc PathTransformFunc
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		opts.Root = DefaultRootDirNAme
	}
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) Has(key string) bool {
	pathKey := s.PathTransformFunc(key)
	_, err := os.Stat(pathKey.FullPath(s.Root))
	return !errors.Is(err, os.ErrNotExist)
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

func (s *Store) Delete(key string) error {
	pathKey := s.PathTransformFunc(key)

	defer func() {
		fmt.Printf("deleted %s from disk", pathKey.Filename)
	}()

	fmt.Println(pathKey.FullPath(s.Root)[5:10])
	return os.RemoveAll(s.Root + "/" + pathKey.FullPath(s.Root)[5:10])
}

func (s *Store) Read(key string) (int64, io.Reader, error) {
	return s.readStream(key)
	// n, f, err := s.readStream(key)
	// if err != nil {
	// 	return 0, nil, err
	// }
	// defer f.Close()

	// buf := new(bytes.Buffer)
	// _, err = io.Copy(buf, f)

	// return n, buf, err
}

func (s *Store) readStream(key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)

	file, err := os.Open(pathKey.FullPath(s.Root))
	if err != nil {
		return 0, nil, err
	}
	f, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	return int64(f.Size()), file, nil
}

func (s *Store) Write(key string, r io.Reader) (int64, error) {
	return s.writeStream(key, r)
}

func (s *Store) writeStream(key string, r io.Reader) (int64, error) {
	pathKey := s.PathTransformFunc(key)

	if err := os.MkdirAll(s.Root+"/"+pathKey.PathName, os.ModePerm); err != nil {
		return 0, err
	}

	fullPath := pathKey.FullPath(s.Root)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}
	return n, nil
}
