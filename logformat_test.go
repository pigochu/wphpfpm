package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unsafe"
)

const (
	testTimeFormat = "2006-01-02 15:04:05 -0700"
)

func formatbuffer(level string, msg string) []byte {
	var b *bytes.Buffer
	b = &bytes.Buffer{}
	t := time.Now()
	b.WriteString(t.Format(testTimeFormat))
	b.WriteString(" [")
	b.WriteString(level)
	b.WriteString("]: ")
	b.WriteString(msg)
	b.WriteByte('\n')
	return b.Bytes()
}

func formatstringadd(level string, msg string) []byte {
	var b string
	t := time.Now()
	b = t.Format(testTimeFormat) + " [" + level + "]: " + msg + "\n"
	return []byte(b)
}

func formatstringaddunsafe(level string, msg string) []byte {
	var s string
	t := time.Now()
	s = t.Format(testTimeFormat) + " [" + level + "]: " + msg + "\n"
	return *(*[]byte)(unsafe.Pointer(&s))
}

func formatstringbuilder(level string, msg string) []byte {
	var builder strings.Builder
	t := time.Now()
	builder.WriteString(t.Format(testTimeFormat))
	builder.WriteString(" [")
	builder.WriteString(level)
	builder.WriteString("]: ")
	builder.WriteString(msg)
	builder.WriteByte('\n')
	return []byte(builder.String())
}

func formatstringbuilderunsafe(level string, msg string) []byte {
	var builder strings.Builder
	t := time.Now()
	builder.WriteString(t.Format(testTimeFormat))
	builder.WriteString(" [")
	builder.WriteString(level)
	builder.WriteString("]: ")
	builder.WriteString(msg)
	builder.WriteByte('\n')
	s := builder.String()
	return *(*[]byte)(unsafe.Pointer(&s))
}

func BenchmarkFormatBuffer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatbuffer("debug", "This is a test message blah blah blah blah blah blah blah blah blah blah ...")
	}
}

func BenchmarkFormatStringAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatstringadd("debug", "This is a test message blah blah blah blah blah blah blah blah blah blah ...")
	}
}

func BenchmarkFormatStringAddUnsafe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatstringaddunsafe("debug", "This is a test message blah blah blah blah blah blah blah blah blah blah ...")
	}
}

func BenchmarkFormatBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatstringbuilder("debug", "This is a test message blah blah blah blah blah blah blah blah blah blah ...")
	}
}

func BenchmarkFormatBuilderUnsafe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatstringbuilderunsafe("debug", "This is a test message blah blah blah blah blah blah blah blah blah blah ...")
	}
}
