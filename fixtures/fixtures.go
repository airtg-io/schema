// Package fixtures holds golden examples of the messages of the schema, for
// the tests of producers (does what they send match?) and consumers (can
// they read what producers send?).
//
// FS holds, under v1/, each example as <name>.binpb (binary encoding: what
// travels on the wire) and, except for unknown-payload, <name>.json (the
// same message as protobuf JSON, for reading). Every file is one
// airtgv1.Envelope:
//
//	metrics             a MetricBatch with every value type and mode
//	inventory           an Inventory
//	metrics-dotted-id   a minimal MetricBatch of a device whose id holds dots
//	metrics-tenant      a MetricBatch with a tenant id and a producer id, as a
//	                    multi-tenant (SaaS) deployment sends; on-premises
//	                    producers leave both empty
//	unknown-payload     an envelope whose payload is unknown to this version,
//	                    as sent by a producer built with a newer schema:
//	                    consumers must ignore it, not fail
//
// Fixtures are append-only: once released, a file never changes and is
// never removed, so that consumers keep proving they read what older
// producers send. A new version of the schema adds new files.
package fixtures

import "embed"

// FS holds the fixture files, under v1/.
//
//go:embed v1
var FS embed.FS
