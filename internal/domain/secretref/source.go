package secretref

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"unicode/utf8"
)

const (
	defaultGenerateLength = 32
	maxGenerateLength     = 512

	alnumChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	digitChars = "0123456789"
)

// Source is what an agent writes at a secret position. Exactly one field is
// set:
//   - "text"                                  → Literal (plain, non-secret value)
//   - {"secret_ref": "app:<id>/variables/X"}  → Ref (copy; the same ref = keep)
//   - {"generate": {"format", "length"}, "name"} → Generate (random value)
//   - {"reuse": "<name>"}                     → Reuse (a value generated under name in the same request)
type Source struct {
	Literal  *string
	Ref      *Ref
	Generate *Generate
	Reuse    string
}

type Generate struct {
	Format string
	Length int
	Name   string
}

func ParseSource(node any) (*Source, error) {
	switch v := node.(type) {
	case string:
		return &Source{Literal: &v}, nil
	case map[string]any:
		return parseSourceObject(v)
	case nil:
		return &Source{Literal: new("")}, nil
	default:
		return nil, fmt.Errorf("expected a string or a value source object, got %T", node)
	}
}

func parseSourceObject(v map[string]any) (*Source, error) {
	if raw, ok := v["secret_ref"]; ok {
		s, _ := raw.(string)
		ref, err := ParseRef(s)
		if err != nil {
			return nil, err
		}
		return &Source{Ref: ref}, nil
	}

	if raw, ok := v["generate"]; ok {
		spec, _ := raw.(map[string]any)
		gen := &Generate{Format: "alnum", Length: defaultGenerateLength}
		if format, ok := spec["format"].(string); ok && format != "" {
			gen.Format = format
		}
		if length, ok := spec["length"].(float64); ok && length > 0 {
			gen.Length = int(length)
		}
		if name, ok := v["name"].(string); ok {
			gen.Name = name
		}
		if gen.Length > maxGenerateLength {
			return nil, fmt.Errorf("generate: length %d exceeds %d", gen.Length, maxGenerateLength)
		}
		switch gen.Format {
		case "alnum", "digits", "hex", "base64url", "uuid":
		default:
			return nil, fmt.Errorf("generate: unknown format %q (alnum, digits, hex, base64url, uuid)", gen.Format)
		}
		return &Source{Generate: gen}, nil
	}

	if raw, ok := v["reuse"]; ok {
		name, _ := raw.(string)
		if name == "" {
			return nil, fmt.Errorf("reuse: empty name")
		}
		return &Source{Reuse: name}, nil
	}

	return nil, fmt.Errorf("unknown value source object: expected secret_ref, generate or reuse")
}

// Chars is the length of the value a generate source will produce.
func (g *Generate) Chars() int {
	if g.Format == "uuid" {
		return 36
	}
	return g.Length
}

func (g *Generate) Value() (string, error) {
	switch g.Format {
	case "alnum":
		return randString(alnumChars, g.Length)
	case "digits":
		return randString(digitChars, g.Length)
	case "hex":
		raw := make([]byte, (g.Length+1)/2)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("rand.Read: %w", err)
		}
		return hex.EncodeToString(raw)[:g.Length], nil
	case "base64url":
		raw := make([]byte, g.Length)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("rand.Read: %w", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)[:g.Length], nil
	case "uuid":
		return uuid4()
	default:
		return "", fmt.Errorf("generate: unknown format %q", g.Format)
	}
}

func uuid4() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

// randString samples charset uniformly (rejection sampling, no modulo bias).
func randString(charset string, length int) (string, error) {
	limit := 256 - 256%len(charset)
	result := make([]byte, 0, length)
	buf := make([]byte, length*2)

	for len(result) < length {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("rand.Read: %w", err)
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			result = append(result, charset[int(b)%len(charset)])
			if len(result) == length {
				break
			}
		}
	}

	return string(result), nil
}

// MaskedLiteralChars describes a stored secret without its value.
func MaskedLiteralChars(value string) string {
	return fmt.Sprintf("secret (%d chars)", utf8.RuneCountInString(value))
}
