// Package schema is the AIrtg wire contract between producers (collectors
// such as snmp-poller) and consumers (such as the analyzer).
//
// The messages are defined in proto/airtg/v1 and generated into package
// github.com/airtg-io/schema/airtg/v1. This package holds the rest of the
// contract: the schema version and how messages travel over NATS.
//
// Every message is one binary-encoded airtgv1.Envelope, published with
// JetStream on the subject of its payload, with the envelope id as the
// Nats-Msg-Id header:
//
//	airtg.v1.metrics.<device>    MetricBatch
//	airtg.v1.inventory.<device>  Inventory
//
// where <device> is the device id encoded by Token. See README.md.
package schema

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

//go:embed VERSION
var version string

// Version is the version of this schema (e.g. "1.4.0"): producers put it in
// Envelope.schema_version.
var Version = strings.TrimSpace(version)

// Subjects.
const (
	// SubjectPrefix starts every subject of this major version of the schema.
	SubjectPrefix = "airtg.v1"
	// SubjectAll matches every subject of this major version of the schema.
	SubjectAll = SubjectPrefix + ".>"
	// SubjectMetricsAll matches the subjects of every device's MetricBatch.
	SubjectMetricsAll = SubjectPrefix + ".metrics.*"
	// SubjectInventoryAll matches the subjects of every device's Inventory.
	SubjectInventoryAll = SubjectPrefix + ".inventory.*"
)

// MetricsSubject is the subject of the MetricBatch messages of a device.
func MetricsSubject(deviceID string) string {
	return SubjectPrefix + ".metrics." + Token(deviceID)
}

// InventorySubject is the subject of the Inventory messages of a device.
func InventorySubject(deviceID string) string {
	return SubjectPrefix + ".inventory." + Token(deviceID)
}

// MaxIDLength is the maximum length of an id, in bytes.
const MaxIDLength = 255

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidID reports whether id is a valid id: Device.id, and Envelope.tenant_id
// and Producer.id when they are set.
func ValidID(id string) bool {
	return len(id) <= MaxIDLength && idPattern.MatchString(id)
}

// Token encodes s as one NATS subject token. Letters, digits, '-' and '_'
// are kept; every other byte, '.' included, becomes '~' followed by its two
// lowercase hex digits ("10.0.0.1" -> "10~2e0~2e0~2e1"). The encoding is
// injective, so two device ids never share a subject.
func Token(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isTokenChar(c) {
			b.WriteByte(c)
			continue
		}
		const hex = "0123456789abcdef"
		b.WriteByte('~')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// ParseToken decodes a token made by Token.
func ParseToken(t string) (string, error) {
	var b strings.Builder
	b.Grow(len(t))
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case isTokenChar(c):
			b.WriteByte(c)
		case c == '~' && i+2 < len(t) && isLowerHex(t[i+1]) && isLowerHex(t[i+2]):
			b.WriteByte(unhex(t[i+1])<<4 | unhex(t[i+2]))
			i += 2
		default:
			return "", fmt.Errorf("invalid subject token %q at byte %d", t, i)
		}
	}
	return b.String(), nil
}

func isTokenChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

func isLowerHex(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' }

func unhex(c byte) byte {
	if c <= '9' {
		return c - '0'
	}
	return c - 'a' + 10
}
