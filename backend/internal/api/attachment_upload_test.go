package api

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"
)

type memoryMultipartFile struct {
	*bytes.Reader
}

func (memoryMultipartFile) Close() error { return nil }

func TestDetectUploadedMIMERejectsDeclaredImageWithHTMLContent(t *testing.T) {
	file := memoryMultipartFile{Reader: bytes.NewReader([]byte("<!doctype html><html><body>not an image</body></html>"))}
	header := &multipart.FileHeader{Filename: "photo.png", Header: make(textproto.MIMEHeader)}
	header.Header.Set("Content-Type", "image/png")

	if _, err := detectUploadedMIME(file, header); err == nil {
		t.Fatal("expected MIME mismatch to be rejected")
	}
}

func TestDetectUploadedMIMEAllowsText(t *testing.T) {
	file := memoryMultipartFile{Reader: bytes.NewReader([]byte("plain text document\n"))}
	header := &multipart.FileHeader{Filename: "notes.txt", Header: make(textproto.MIMEHeader)}
	header.Header.Set("Content-Type", "text/plain")

	got, err := detectUploadedMIME(file, header)
	if err != nil {
		t.Fatalf("detectUploadedMIME() error = %v", err)
	}
	if got != "text/plain" {
		t.Fatalf("MIME = %q, want text/plain", got)
	}
}

func TestDetectUploadedMIMERejectsUnknownBinary(t *testing.T) {
	file := memoryMultipartFile{Reader: bytes.NewReader([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05})}
	header := &multipart.FileHeader{Filename: "payload.bin", Header: make(textproto.MIMEHeader)}
	header.Header.Set("Content-Type", "application/octet-stream")

	if _, err := detectUploadedMIME(file, header); err == nil {
		t.Fatal("expected unknown binary upload to be rejected")
	}
}
