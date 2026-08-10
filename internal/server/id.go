package server

import (
	"crypto/rand"
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

//go:embed wordlist.txt
var wordlistFS embed.FS

var defaultGen = mustNewGenerator(wordlistFS)

func GenerateID() string {
	return defaultGen.GenerateID()
}

type Generator struct {
	words []string
}

func NewGenerator(words []string) (*Generator, error) {
	if len(words) < 3 {
		return nil, fmt.Errorf("kariz: wordlist too small: need at least 3, got %d", len(words))
	}
	return &Generator{words: words}, nil
}

func (g *Generator) GenerateID() string {
	n := uint32(len(g.words))
	i1 := g.randIndex(n)
	i2 := g.randIndex(n)
	for i2 == i1 {
		i2 = g.randIndex(n)
	}
	i3 := g.randIndex(n)
	for i3 == i1 || i3 == i2 {
		i3 = g.randIndex(n)
	}
	return g.words[i1] + "." + g.words[i2] + "." + g.words[i3]
}

func (g *Generator) randIndex(n uint32) uint32 {
	var b [4]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		panic(fmt.Sprintf("kariz: crypto/rand failed: %v", err))
	}
	return binary.LittleEndian.Uint32(b[:]) % n
}

func mustNewGenerator(fsys embed.FS) *Generator {
	data, err := fsys.ReadFile("wordlist.txt")
	if err != nil {
		panic(fmt.Sprintf("kariz: failed to read embedded wordlist: %v", err))
	}
	words := strings.Fields(string(data))
	g, err := NewGenerator(words)
	if err != nil {
		panic(fmt.Sprintf("kariz: bad wordlist: %v", err))
	}
	return g
}
