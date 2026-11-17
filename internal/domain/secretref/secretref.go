// Package secretref hides secret values of configuration objects from MCP
// agents and resolves the declarative value sources agents write in their
// place.
//
// Objects are handled as generic JSON trees (the result of json.Unmarshal into
// any), so one walker covers root, apps and endpoints. A secret leaf is shown
// to an agent as {"secret_ref": "<ref>", "chars": n}; sending that object back
// (at the same or any other secret position) copies the value the ref points
// to, so "keep" and "copy" are the same operation.
package secretref

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	KindRoot     = "root"
	KindApp      = "app"
	KindEndpoint = "endpoint"
)

// secretPaths lists the positions of secret values inside root/app/endpoint
// JSON; "*" matches any map key or array index. Must stay in sync with the
// Redacted() methods of the domain models.
var secretPaths = [][]string{
	{"auth", "methods", "*", "basic", "users", "*", "password"},
	{"auth", "methods", "*", "api_key", "keys", "*", "key"},
	{"variables", "*"},
	{"backend", "headers", "*"},
	{"backend", "query_params", "*"},
}

// IsSecretPath reports whether a full path addresses a secret leaf.
func IsSecretPath(path []string) bool {
	for _, pattern := range secretPaths {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

// isSecretPrefix reports whether a secret leaf may lie at or below path.
func isSecretPrefix(path []string) bool {
	for _, pattern := range secretPaths {
		if len(path) <= len(pattern) && matchPath(pattern[:len(path)], path) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}
	for i := range pattern {
		if pattern[i] != "*" && pattern[i] != path[i] {
			return false
		}
	}
	return true
}

// ── Ref ─────────────────────────────────────────────────

// Ref addresses one secret value: the owning object and the JSON pointer of
// the leaf inside it.
type Ref struct {
	Kind string
	Id   string
	Path []string
}

// String renders "app:<id>/variables/TOKEN", "root:/auth/...".
func (r *Ref) String() string {
	var b strings.Builder
	b.WriteString(r.Kind)
	b.WriteString(":")
	b.WriteString(r.Id)
	for _, seg := range r.Path {
		b.WriteString("/")
		b.WriteString(escapeSegment(seg))
	}
	return b.String()
}

// Object is the "kind:id" key of the object the ref points into.
func (r *Ref) Object() string {
	return ObjectKey(r.Kind, r.Id)
}

func ObjectKey(kind, id string) string {
	return kind + ":" + id
}

func ParseRef(s string) (*Ref, error) {
	kind, rest, ok := strings.Cut(s, ":")
	if !ok {
		return nil, fmt.Errorf("secret_ref %q: expected <kind>:<id>/<path>", s)
	}
	id, pointer, _ := strings.Cut(rest, "/")
	if pointer == "" {
		return nil, fmt.Errorf("secret_ref %q: empty path", s)
	}

	switch kind {
	case KindRoot:
		if id != "" {
			return nil, fmt.Errorf("secret_ref %q: root has no id", s)
		}
	case KindApp, KindEndpoint:
		if id == "" {
			return nil, fmt.Errorf("secret_ref %q: empty id", s)
		}
	default:
		return nil, fmt.Errorf("secret_ref %q: unknown kind %q", s, kind)
	}

	segs := strings.Split(pointer, "/")
	for i := range segs {
		segs[i] = unescapeSegment(segs[i])
	}
	if !IsSecretPath(segs) {
		return nil, fmt.Errorf("secret_ref %q: path does not address a secret value", s)
	}

	return &Ref{Kind: kind, Id: id, Path: segs}, nil
}

func escapeSegment(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func unescapeSegment(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

// ── Masking ─────────────────────────────────────────────

// Masked is what an agent sees in place of a secret value.
type Masked struct {
	SecretRef string `json:"secret_ref"`
	Chars     int    `json:"chars"`
}

// ToTree converts a domain object to a generic JSON tree.
func ToTree(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}
	var tree any
	if err = json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return tree, nil
}

// Mask returns a copy of tree with every secret leaf replaced by Masked.
func Mask(kind, id string, tree any) any {
	result, _ := walk(tree, nil, func(path []string, node any) (any, error) {
		value, _ := node.(string)
		ref := &Ref{Kind: kind, Id: id, Path: path}
		return map[string]any{
			"secret_ref": ref.String(),
			"chars":      utf8.RuneCountInString(value),
		}, nil
	})
	return result
}

// Lookup returns the string at path, if any.
func Lookup(tree any, path []string) (string, bool) {
	node := tree
	for _, seg := range path {
		switch v := node.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return "", false
			}
			node = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return "", false
			}
			node = v[idx]
		default:
			return "", false
		}
	}
	value, ok := node.(string)
	return value, ok
}

// Rewrite returns a copy of tree where every secret leaf is replaced by fn's
// result.
func Rewrite(tree any, fn func(path []string, node any) (any, error)) (any, error) {
	return walk(tree, nil, fn)
}

func walk(node any, path []string, fn func(path []string, node any) (any, error)) (any, error) {
	if len(path) > 0 && IsSecretPath(path) {
		return fn(append([]string(nil), path...), node)
	}
	if len(path) > 0 && !isSecretPrefix(path) {
		return node, nil
	}

	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, child := range v {
			next, err := walk(child, append(path, k), fn)
			if err != nil {
				return nil, err
			}
			result[k] = next
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			next, err := walk(child, append(path, strconv.Itoa(i)), fn)
			if err != nil {
				return nil, err
			}
			result[i] = next
		}
		return result, nil
	default:
		return node, nil
	}
}

// Version is a short fingerprint of an object's stored state; an agent passes
// it back with an update so a change made meanwhile is not overwritten.
func Version(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8]), nil
}

// FormatPath renders a path for people: auth.methods[0].basic.users[1].password.
func FormatPath(path []string) string {
	var b strings.Builder
	for i, seg := range path {
		if _, err := strconv.Atoi(seg); err == nil && i > 0 {
			b.WriteString("[" + seg + "]")
			continue
		}
		if i > 0 {
			b.WriteString(".")
		}
		b.WriteString(seg)
	}
	return b.String()
}

// MaskAnonymous masks secrets of a computed object (e.g. the effective config
// after inheritance) whose values cannot be addressed by a ref.
func MaskAnonymous(tree any) any {
	result, _ := walk(tree, nil, func(_ []string, node any) (any, error) {
		value, _ := node.(string)
		return map[string]any{"chars": utf8.RuneCountInString(value)}, nil
	})
	return result
}
