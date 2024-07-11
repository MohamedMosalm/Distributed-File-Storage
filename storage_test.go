package main

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"math/big"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	key := "mohamedmosalm"
	pathKey := CASPathTransformFunc(key)
	expectedPathName := "4366d/d59e7/8c689/62e94/415ff/63fc9/7fb41/c86ba"
	expectedFilename := "4366dd59e78c68962e94415ff63fc97fb41c86ba"

	if pathKey.PathName != expectedPathName {
		t.Errorf("have %s want %s", pathKey.PathName, expectedPathName)
	}

	if pathKey.Filename != expectedFilename {
		t.Errorf("expected path name = %s, got %s", pathKey.Filename, expectedFilename)
	}
}

func TestStore(t *testing.T) {
	store := newStore()
	defer teardown(t, store)

	for i := 0; i < 50; i++ {
		key, err := randomString(i)
		if err != nil {
			t.Errorf("Error generating random string: %s", err)
		}
		data := []byte(key)

		if err := store.writeStream(key, bytes.NewReader(data)); err != nil {
			t.Error(err)
		}

		if ok := store.Has(key); !ok {
			t.Errorf("expected to have key %s", key)
		}

		r, err := store.Read(key)

		if err != nil {
			t.Error(err)
		}

		b, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}

		if string(b) != string(data) {
			t.Errorf("expected %d bytes, got %d", data, b)
		}

		if err := store.Delete(key); err != nil {
			t.Error(err)
		}

		if ok := store.Has(key); ok {
			t.Errorf("expected to not have key %s", key)
		}
	}
}

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}
