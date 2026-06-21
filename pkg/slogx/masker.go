package slogx

import (
	"fmt"
	"strings"
)

// MaskType defines the category of data being masked.
type MaskType int

const (
	// MaskDefault replaces the value with [MASKED].
	MaskDefault MaskType = iota
	// MaskEmail redacts email addresses while preserving parts of the username and domain.
	MaskEmail
	// MaskPhone redacts phone numbers while preserving the prefix and last few digits.
	MaskPhone
	// MaskCard redacts credit card numbers, showing only the first and last four digits.
	MaskCard
	// MaskSecret completely hides the value with [SECRET].
	MaskSecret
)

// Masker is the interface for custom masking logic.
type Masker interface {
	Mask(value any, mType MaskType) any
}

// DefaultMasker provides a standard implementation of Masker.
type DefaultMasker struct{}

// MaskMap associates attribute keys with masking strategies.
type MaskMap map[string]MaskType

// MaskRules is a fluent builder for masking configuration.
type MaskRules struct {
	rules MaskMap
}

// NewMaskRules creates a new MaskRules builder.
func NewMaskRules() *MaskRules {
	return &MaskRules{rules: make(MaskMap)}
}

// Add registers a key with a masking strategy.
func (r *MaskRules) Add(key string, mType MaskType) *MaskRules {
	r.rules[key] = mType
	return r
}

// Keys returns the underlying MaskMap.
func (r *MaskRules) Keys() MaskMap {
	return r.rules
}

// Mask applies the appropriate redaction strategy for the given MaskType.
func (m *DefaultMasker) Mask(value any, mType MaskType) any {
	valStr := fmt.Sprintf("%v", value)
	switch mType {
	case MaskEmail:
		return maskEmail(valStr)
	case MaskPhone:
		return maskPhone(valStr)
	case MaskCard:
		return maskCard(valStr)
	case MaskSecret:
		return "[SECRET]"
	default:
		return "[MASKED]"
	}
}

func maskEmail(s string) string {
	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	user, domain := parts[0], parts[1]
	if len(user) <= 2 {
		return user[:1] + "***@" + domain
	}
	return user[:2] + "***" + user[len(user)-1:] + "@" + domain
}

func maskPhone(s string) string {
	if len(s) < 8 {
		return "***"
	}
	return s[:4] + "*******" + s[len(s)-3:]
}

func maskCard(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) < 12 {
		return "**** **** ****"
	}
	return s[:4] + " **** **** " + s[len(s)-4:]
}
