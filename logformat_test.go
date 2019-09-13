package main

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

const (
	testTimeFormat = "2006-01-02 15:04:05 -0700"
)

type BufferFormatter struct {
	timeFormat string
}

func (f *BufferFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	b.WriteString(entry.Time.Format(f.timeFormat))
	b.WriteString(" [")
	b.WriteString(entry.Level.String())
	b.WriteString("]: ")
	b.WriteString(entry.Message)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

type StringAddFormatter struct {
	timeFormat string
}

func (f *StringAddFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b string
	b = entry.Time.Format(f.timeFormat) + " [" + entry.Level.String() + "]: " + entry.Message + "\n"
	return []byte(b), nil
}

type StringAddUnsafeFormatter struct {
	timeFormat string
}

func (f *StringAddUnsafeFormatter) Format(entry *log.Entry) ([]byte, error) {
	var s string
	s = entry.Time.Format(f.timeFormat) + " [" + entry.Level.String() + "]: " + entry.Message + "\n"
	return *(*[]byte)(unsafe.Pointer(&s)), nil
}

type StringBuilderFormatter struct {
	timeFormat string
}

func (f *StringBuilderFormatter) Format(entry *log.Entry) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString(entry.Time.Format(f.timeFormat))
	builder.WriteString(" [")
	builder.WriteString(entry.Level.String())
	builder.WriteString("]: ")
	builder.WriteString(entry.Message)
	builder.WriteByte('\n')
	return []byte(builder.String()), nil
}

type StringBuilderUnsafeFormatter struct {
	timeFormat string
}

func (f *StringBuilderUnsafeFormatter) Format(entry *log.Entry) ([]byte, error) {
	var builder strings.Builder

	builder.WriteString(entry.Time.Format(f.timeFormat))
	builder.WriteString(" [")
	builder.WriteString(entry.Level.String())
	builder.WriteString("]: ")
	builder.WriteString(entry.Message)
	builder.WriteByte('\n')
	s := builder.String()
	return *(*[]byte)(unsafe.Pointer(&s)), nil
}

func BenchmarkFormatBuffer(b *testing.B) {
	formatter := &BufferFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		log.Debugf("%d This is a test message blah blah blah blah blah blah blah blah blah blah ...", i)
	}
}

func BenchmarkFormatStringAdd(b *testing.B) {
	formatter := &StringAddFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		log.Debugf("%d This is a test message blah blah blah blah blah blah blah blah blah blah ...", i)
	}
}

func BenchmarkFormatStringAddUnsafe(b *testing.B) {
	formatter := &StringAddUnsafeFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		log.Debugf("%d This is a test message blah blah blah blah blah blah blah blah blah blah ...", i)
	}
}

func BenchmarkFormatBuilder(b *testing.B) {
	formatter := &StringBuilderFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		log.Debugf("%d This is a test message blah blah blah blah blah blah blah blah blah blah ...", i)
	}
}

func BenchmarkFormatBuilderUnsafe(b *testing.B) {
	formatter := &StringBuilderUnsafeFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		log.Debugf("%d This is a test message blah blah blah blah blah blah blah blah blah blah ...", i)
	}
}
