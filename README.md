# AIrtg schema

The wire contract of AIrtg: what collectors such as
[snmp-poller](https://github.com/airtg-io/snmp-poller) send, and what consumers such as the
analyzer receive.

- **Protobuf messages** in [`proto/airtg/v1`](proto/airtg/v1), every element documented: the
  comments there are the reference.
- **Generated Go code**, committed: `import airtgv1 "github.com/airtg-io/schema/airtg/v1"`.
  Nothing to install or generate on the consumer side.
- **NATS transport binding** (subjects, headers, message ids) in package
  `github.com/airtg-io/schema`.
- **Golden fixtures** for the tests of producers and consumers, in package
  `github.com/airtg-io/schema/fixtures`.
- **Compatibility checked in CI** against the last release: binary and JSON encodings, and
  unchanged fixtures.

Copyright 2026 AIrtg Solutions, inc. Licensed under the Apache License 2.0 (see
[LICENSE](LICENSE) and [NOTICE](NOTICE)), so anyone can produce or consume AIrtg data, in any
program, under any license.

## Contents

- [Messages](#messages)
- [Transport over NATS](#transport-over-nats)
- [Rules every producer and consumer follows](#rules-every-producer-and-consumer-follows)
- [Versioning and compatibility](#versioning-and-compatibility)
- [Using the schema](#using-the-schema)
- [Development](#development)

## Messages

Every message is one `Envelope`: a common header and exactly one payload.

| Message | Purpose |
|---------|---------|
| `Envelope` | `id` (unique, for deduplication), `schema_version`, `producer`, `time`, `tenant_id` (empty on premises), and the payload |
| `Producer` | `type` (e.g. `snmp-poller`), `version`, `hostname`, and `id` (assigned in SaaS deployments, empty on premises) |
| `MetricBatch` | Samples polled from one `Device`, with its poll `interval` |
| `Sample` | One value: `capability`, `metric`, `table` and `index` for table rows, `labels` naming the row, `mode` (raw, rate, volume), `source_type`, `unit`, `time`, and an int, uint, double or string `value` |
| `Inventory` | What discovery found on one `Device`: `capabilities`, `scalars` (sysName, sysDescr, ...) and `columns` (ifName, ifAlias, ... by row index). Each one replaces the previous one entirely |
| `Device` | `id` (the correlation key, see below) and `address` |

Events (traps, alarms), logs (syslog) and topology will be added as new payloads of the
envelope.

## Transport over NATS

Producers publish with **JetStream**, each message being one binary-encoded `Envelope`:

| Subject | Payload |
|---------|---------|
| `airtg.v1.metrics.<device>` | `MetricBatch` |
| `airtg.v1.inventory.<device>` | `Inventory` |

- `<device>` is the device id encoded as one subject token by `schema.Token`: letters, digits,
  `-` and `_` are kept, any other byte becomes `~` and two lowercase hex digits
  (`10.0.0.1-161` -> `10~2e0~2e0~2e1-161`). Use `schema.MetricsSubject(id)` and
  `schema.InventorySubject(id)`; `schema.SubjectAll` (`airtg.v1.>`) matches everything.
- The `Nats-Msg-Id` header holds `Envelope.id`, so that JetStream drops a message published
  twice within its duplicate window.
- A message never exceeds the server's `max_payload` (1 MB by default). Producers split the
  samples of a poll into several `MetricBatch` messages when needed. An `Inventory` is never
  split, so servers receiving inventories of large devices should raise `max_payload`
  (8 MB is plenty).
- Streams belong to the consumer. Recommended: one stream for `airtg.v1.metrics.*` limited by
  age, and one for `airtg.v1.inventory.*` with `max_msgs_per_subject: 1`, so that a consumer
  starting at any time finds the latest inventory of every device.

## Rules every producer and consumer follows

**Device identity.** `Device.id` is the device's id from the producer's point of view: for
snmp-poller, the id configured for the device (by default `<address>-<port>`). It is the key
tying together what is sent about the device: today's metrics and inventory, later its events,
logs and topology. It is stable over time, producers collecting from the same device should
use the same id, and it matches `^[A-Za-z0-9][A-Za-z0-9._-]*$`, at most 255 bytes
(`schema.ValidID`). The address is informational only.

It is not meant to stay the only identity: others may be added to `Device` as new fields, such
as a cluster id for the members of a clustered device, or ids found by discovery (a serial
number, ...) rather than configured. Consumers should not assume `id` is the only identity a
device will ever carry.

**Tenants and producers.** `Envelope.tenant_id` and `Producer.id` are empty in on-premises
deployments. In a deployment serving several tenants (SaaS), the producer id identifies a
producer installation for good (assigned at registration, or a serial number), unlike its
`hostname`. Both follow the device id rule when set.

**`tenant_id` is for debugging and logging only.** It is what the producer *says*, so it is
never used for authorization, isolation or routing. The tenant a message really belongs to is
the one of the credentials the producer authenticated with (e.g. its NATS account, which also
keeps tenants' subjects apart). Ids are unique within that tenant: a device is identified by
(authenticated tenant, device id), a producer by (authenticated tenant, producer id).

**Delivery is at least once.** A producer's disk buffer may send a message again long after
the duplicate window. Consumers tolerate duplicates (`Envelope.id` identifies them) and rely
on `Sample.time`, not on arrival order.

**Time.** Every timestamp is an absolute instant (protobuf `Timestamp`): `Sample.time` is when
the device answered, `Envelope.time` when the message was built, `Inventory.discovered_at` when
the discovery ran.

**Values.** `MODE_RAW` keeps the type the device's value maps to (`uint_value` for counters,
gauges and time ticks, `string_value` for octet strings, `int_value` or `double_value` for
formulas). `MODE_RATE` (per second) and `MODE_VOLUME` (increase since the previous poll) are
always `double_value`, counter wraps and resets already handled.

**Units.** Every numeric value carries its unit; only a string value may have none.
Collectors send values as close to their source as possible, in the unit the source uses,
and never normalize across products: a device reporting kilobytes is sent in `kilobyte`, one
reporting bytes in `byte`, and consumers convert if they need to.

- Units are lowercase singular words of the vocabulary `schema.Units`, the same everywhere:
  `percent`, never `%` or `pct`. Validate with `schema.ValidUnit`. A unit a collector needs and
  the vocabulary lacks is added to it in a new minor version, never invented.

  | Kind | Units |
  |------|-------|
  | Data | `bit`, `kilobit`, `megabit`, `gigabit`, `byte`, `kilobyte`, `megabyte`, `gigabyte`, `terabyte`, `octet`, `allocation unit` |
  | Network | `packet`, `frame`, `segment` (TCP), `datagram` (UDP) |
  | Time | `nanosecond`, `microsecond`, `millisecond`, `second`, `minute`, `hour`, `day` |
  | Ratio | `percent` |
  | Things counted | `connection`, `entry`, `error`, `event`, `message`, `process`, `request`, `route`, `session`, `thread`, `user` |
  | Physical | `celsius`, `fahrenheit`, `kelvin`, `milliwatt`, `watt`, `kilowatt`, `watt-hour`, `kilowatt-hour`, `volt-ampere`, `millivolt`, `volt`, `milliampere`, `ampere`, `hertz`, `kilohertz`, `megahertz`, `gigahertz`, `rpm`, `decibel`, `dbm` |
  | Codes | `enum`: a number that names a state or a thing rather than measuring a quantity (ifOperStatus 1 up, 2 down, ...; a process id) |

- The unit names what the source counts: IF-MIB's `ifInOctets` is in `octet`, not `byte`.
- A rate appends `/s` to the unit of its counter (`octet/s`), and so does a source value that is
  a rate itself (`ifSpeed` in `bit/s`, `ifHighSpeed` in `megabit/s`).
- A formula's value has the unit of its inputs, or the one their combination gives:
  `hrStorageSize` (`allocation unit`) times `hrStorageAllocationUnits` (`byte`) is in `byte`, a
  ratio times 100 in `percent`. A formula that would only scale a value (kilobytes to bytes)
  is not written: the source value is sent as it is.
- One exception for time: SNMP time ticks (hundredths of a second) are converted to `second`,
  the unit everyone uses for durations; `centisecond` is not in the vocabulary.

**Rows.** A table sample has both `table` and `index`, a scalar neither. `index` may change
when a device reboots; `labels` (also in the inventory's `columns`) name the row for people
and for matching.

**Inventory.** Sent after every discovery of a device, changed or not. Each one replaces the
previous one for the device entirely.

**Forward compatibility.** Consumers ignore unknown fields, envelopes without a known payload
(counting them), and samples whose `mode` they do not know. They never fail on data from a
newer producer.

## Versioning and compatibility

The version is in [`VERSION`](VERSION) (`MAJOR.MINOR.PATCH`) and in `schema.Version`; producers
put it in `Envelope.schema_version`.

- **Minor versions only add**: new fields, payloads, enum values and messages. A producer and a
  consumer of any two 1.x versions understand each other.
- **Nothing incompatible ever happens within `airtg.v1`**: no field removed, renumbered,
  retyped or renamed (renaming breaks JSON). A change that cannot be additive goes into a new
  package `airtg/v2` with its own subjects (`airtg.v2.>`), next to v1, so producers and
  consumers can move one at a time. The Go module itself stays `v1`.
- **Fixtures are append-only**: a released fixture never changes, so consumers keep proving they
  read what older producers send.
- CI enforces both against the last release tag with `make breaking`. Before 1.0.0 the schema may
  still change, and incompatibilities are only reported.

## Using the schema

```sh
go get github.com/airtg-io/schema@latest
```

**Producing**:

```go
env := &airtgv1.Envelope{
	Id:            uuid.NewString(),
	SchemaVersion: schema.Version,
	Producer:      &airtgv1.Producer{Type: "snmp-poller", Version: version, Hostname: hostname},
	Time:          timestamppb.Now(),
	Payload:       &airtgv1.Envelope_Metrics{Metrics: batch},
}
data, err := proto.Marshal(env)
// ...
_, err = js.Publish(ctx, schema.MetricsSubject(batch.GetDevice().GetId()), data,
	jetstream.WithMsgID(env.GetId()))
```

**Consuming**:

```go
var env airtgv1.Envelope
if err := proto.Unmarshal(msg.Data(), &env); err != nil {
	// not an envelope: count, log, and acknowledge it so it is not redelivered
}
switch p := env.GetPayload().(type) {
case *airtgv1.Envelope_Metrics:
	// p.Metrics
case *airtgv1.Envelope_Inventory:
	// p.Inventory
default:
	// a payload from a newer schema: ignore it
}
```

**Testing** against the fixtures (see the `fixtures` package documentation for the list):

```go
bin, err := fs.ReadFile(fixtures.FS, "v1/metrics.binpb")
// producer: what it encodes for the same input decodes to the same message (proto.Equal)
// consumer: it reads every file of fixtures.FS, including unknown-payload.binpb, without error
```

## Development

Everything runs through `make`, with tools pinned as Go tools: nothing to install but Go.

| Command | What it does |
|---------|--------------|
| `make` / `make check` | gofmt and `buf format`, `buf lint`, generated code up to date, `go.mod` tidy, vet, tests |
| `make generate` | regenerate `airtg/` from `proto/` (`airtg/` is wiped: never edit it by hand) |
| `make fixtures` | write the files of fixtures added to `fixtures/fixtures_test.go` |
| `make breaking` | compare with the last release tag |

Changing the schema:

1. Edit `proto/airtg/v1/*.proto`: new fields get new numbers, and every element gets a comment.
2. `make generate`, then add a fixture showing the new data in `fixtures/fixtures_test.go` and
   run `make fixtures`.
3. Bump `VERSION` (minor for an addition, patch for comments only) and run `make`.

**Releases**: CI (`.github/workflows/go.yml`) checks every push and pull request. A push to
`main` whose `VERSION` has no tag `v<VERSION>` yet creates that tag and its GitHub release:
consumers then `go get` it. Dependabot proposes updates of the Go dependencies and of the
GitHub Actions every week. A protobuf update changes the generated code's header, so run
`make generate` and commit on its branch.
