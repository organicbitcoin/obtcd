// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"os"
	"time"
)

type countingWriter struct {
	w    io.Writer
	hash hash.Hash
	n    int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	if w.hash != nil {
		_, _ = w.hash.Write(p)
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

type jsonlGzipWriter struct {
	file      *os.File
	gz        *gzip.Writer
	rawHash   hash.Hash
	fileHash  hash.Hash
	path      string
	rowCount  int64
	byteCount int64
}

func newJSONLGzipWriter(path string) (*jsonlGzipWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	fileHash := sha256.New()
	cw := &countingWriter{w: f, hash: fileHash}
	gz := gzip.NewWriter(cw)
	gz.Name = path
	gz.ModTime = time.Unix(0, 0)
	return &jsonlGzipWriter{
		file:     f,
		gz:       gz,
		rawHash:  sha256.New(),
		fileHash: fileHash,
		path:     path,
	}, nil
}

func (w *jsonlGzipWriter) WriteJSON(v interface{}) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := w.rawHash.Write(line); err != nil {
		return err
	}
	if _, err := w.gz.Write(line); err != nil {
		return err
	}
	w.rowCount++
	w.byteCount += int64(len(line))
	return nil
}

func (w *jsonlGzipWriter) Close() error {
	if err := w.gz.Close(); err != nil {
		_ = w.file.Close()
		return err
	}
	return w.file.Close()
}

func (w *jsonlGzipWriter) SHA256() string {
	return hex.EncodeToString(w.rawHash.Sum(nil))
}

func (w *jsonlGzipWriter) FileSHA256() string {
	return hex.EncodeToString(w.fileHash.Sum(nil))
}
