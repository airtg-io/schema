package fixtures_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/airtg-io/schema"
	airtgv1 "github.com/airtg-io/schema/airtg/v1"
	"github.com/airtg-io/schema/fixtures"
)

// -update writes the files of the fixtures that have none. It never rewrites
// an existing file: fixtures are append-only.
var update = flag.Bool("update", false, "write the files of new fixtures")

// fixture is one example. Exactly one of msg and raw is set: raw is for
// bytes no Envelope of this version can produce.
type fixture struct {
	name string
	msg  *airtgv1.Envelope
	raw  func(t *testing.T) []byte
	// check verifies what consumers must see when decoding raw.
	check func(t *testing.T, env *airtgv1.Envelope)
}

// t0 is the time of every fixture.
var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func ts(offset time.Duration) *timestamppb.Timestamp { return timestamppb.New(t0.Add(offset)) }

func producer() *airtgv1.Producer {
	return &airtgv1.Producer{Type: "snmp-poller", Version: "0.3.0", Hostname: "poller-1.example.net"}
}

var coreSw1 = &airtgv1.Device{Id: "core-sw1", Address: "192.0.2.10"}

var ifLabels = map[string]string{"ifDescr": "GigabitEthernet0/1", "ifName": "Gi0/1"}

var all = []fixture{
	{
		name: "metrics",
		msg: &airtgv1.Envelope{
			Id:            "6f1c2a8e-4b7d-4c0e-9a51-0d3e2f7b8c91",
			SchemaVersion: "0.1.0",
			Producer:      producer(),
			Time:          ts(2 * time.Second),
			Payload: &airtgv1.Envelope_Metrics{Metrics: &airtgv1.MetricBatch{
				Device:   coreSw1,
				Interval: durationpb.New(time.Minute),
				Samples: []*airtgv1.Sample{
					{
						Capability: "system", Metric: "sysUpTime", Mode: airtgv1.Mode_MODE_RAW,
						SourceType: "TimeTicks", Time: ts(0),
						Value: &airtgv1.Sample_UintValue{UintValue: 864000000},
					},
					{
						Capability: "interfaces", Metric: "ifHCInOctets", Table: "ifTable", Index: "1",
						Labels: ifLabels, Mode: airtgv1.Mode_MODE_RAW, SourceType: "Counter64",
						Unit: "bytes", Time: ts(time.Second),
						Value: &airtgv1.Sample_UintValue{UintValue: 18446744073709551000},
					},
					{
						Capability: "interfaces", Metric: "ifHCInOctets", Table: "ifTable", Index: "1",
						Labels: ifLabels, Mode: airtgv1.Mode_MODE_RATE, SourceType: "Counter64",
						Unit: "bytes/s", Time: ts(time.Second),
						Value: &airtgv1.Sample_DoubleValue{DoubleValue: 1250000.5},
					},
					{
						Capability: "interfaces", Metric: "ifHCInOctets", Table: "ifTable", Index: "1",
						Labels: ifLabels, Mode: airtgv1.Mode_MODE_VOLUME, SourceType: "Counter64",
						Unit: "bytes", Time: ts(time.Second),
						Value: &airtgv1.Sample_DoubleValue{DoubleValue: 75000030},
					},
					{
						Capability: "host-resources", Metric: "hrStorageSizeBytes", Table: "hrStorageTable",
						Index: "1", Labels: map[string]string{"hrStorageDescr": "Physical memory"},
						Mode: airtgv1.Mode_MODE_RAW, SourceType: "Formula", Unit: "bytes", Time: ts(time.Second),
						Value: &airtgv1.Sample_IntValue{IntValue: 17179869184},
					},
					{
						Capability: "host-resources", Metric: "hrStorageUsedPercent", Table: "hrStorageTable",
						Index: "1", Labels: map[string]string{"hrStorageDescr": "Physical memory"},
						Mode: airtgv1.Mode_MODE_RAW, SourceType: "Formula", Unit: "percent", Time: ts(time.Second),
						Value: &airtgv1.Sample_DoubleValue{DoubleValue: 42.5},
					},
					{
						Capability: "entity", Metric: "entPhysicalSoftwareRev", Table: "entPhysicalTable",
						Index: "1", Labels: map[string]string{"entPhysicalName": "Chassis"},
						Mode: airtgv1.Mode_MODE_RAW, SourceType: "OctetString", Time: ts(time.Second),
						Value: &airtgv1.Sample_StringValue{StringValue: "17.9.4"},
					},
				},
			}},
		},
	},
	{
		name: "inventory",
		msg: &airtgv1.Envelope{
			Id:            "0b8e5d43-7a1f-4f62-b3c9-5e2a6d14f0a7",
			SchemaVersion: "0.1.0",
			Producer:      producer(),
			Time:          ts(3 * time.Second),
			Payload: &airtgv1.Envelope_Inventory{Inventory: &airtgv1.Inventory{
				Device:       coreSw1,
				DiscoveredAt: ts(0),
				Capabilities: []string{"entity", "host-resources", "interfaces", "system"},
				Scalars: map[string]string{
					"sysName":     "core-sw1.example.net",
					"sysDescr":    "Cisco IOS XE Software, Version 17.09.04",
					"sysObjectID": ".1.3.6.1.4.1.9.1.2494",
					"sysLocation": "DC1, row 4, rack 12",
					"sysContact":  "noc@example.net",
				},
				Columns: map[string]*airtgv1.Column{
					"ifDescr": {Rows: map[string]string{"1": "GigabitEthernet0/1", "2": "GigabitEthernet0/2"}},
					"ifName":  {Rows: map[string]string{"1": "Gi0/1", "2": "Gi0/2"}},
					"ifAlias": {Rows: map[string]string{"1": "uplink to core-sw2", "2": ""}},
				},
			}},
		},
	},
	{
		name: "metrics-dotted-id",
		msg: &airtgv1.Envelope{
			Id:            "c4a9e0f2-1d38-4b5a-8e67-93f0b2d1a6c5",
			SchemaVersion: "0.1.0",
			Producer:      producer(),
			Time:          ts(time.Second),
			Payload: &airtgv1.Envelope_Metrics{Metrics: &airtgv1.MetricBatch{
				Device:   &airtgv1.Device{Id: "10.0.0.1-161", Address: "10.0.0.1"},
				Interval: durationpb.New(5 * time.Minute),
				Samples: []*airtgv1.Sample{{
					Capability: "system", Metric: "sysUpTime", Mode: airtgv1.Mode_MODE_RAW,
					SourceType: "TimeTicks", Time: ts(0),
					Value: &airtgv1.Sample_UintValue{UintValue: 4200},
				}},
			}},
		},
	},
	{
		name: "metrics-tenant",
		msg: &airtgv1.Envelope{
			Id:            "8a3f6e21-9c4b-4d7a-b058-1e2f3a4b5c6d",
			SchemaVersion: "0.1.0",
			Producer: &airtgv1.Producer{
				Type: "snmp-poller", Version: "0.3.0", Hostname: "poller-7f3k2", Id: "prd-01JC9X4M7Q",
			},
			Time:     ts(time.Second),
			TenantId: "acme-corp",
			Payload: &airtgv1.Envelope_Metrics{Metrics: &airtgv1.MetricBatch{
				Device:   coreSw1,
				Interval: durationpb.New(time.Minute),
				Samples: []*airtgv1.Sample{{
					Capability: "system", Metric: "sysUpTime", Mode: airtgv1.Mode_MODE_RAW,
					SourceType: "TimeTicks", Time: ts(0),
					Value: &airtgv1.Sample_UintValue{UintValue: 864000000},
				}},
			}},
		},
	},
	{
		name: "unknown-payload",
		raw: func(t *testing.T) []byte {
			b, err := proto.Marshal(&airtgv1.Envelope{
				Id:            "5d2b7c19-e8a4-4f03-a6b1-2c9d8e7f6a50",
				SchemaVersion: "9.0.0",
				Producer:      &airtgv1.Producer{Type: "future-collector", Version: "1.0.0", Hostname: "fc-1"},
				Time:          ts(0),
			})
			if err != nil {
				t.Fatal(err)
			}
			// Field 99 of the payload oneof, as a newer schema could add it:
			// a message holding one string.
			inner := protowire.AppendTag(nil, 1, protowire.BytesType)
			inner = protowire.AppendString(inner, "from a newer schema")
			b = protowire.AppendTag(b, 99, protowire.BytesType)
			return protowire.AppendBytes(b, inner)
		},
		check: func(t *testing.T, env *airtgv1.Envelope) {
			if env.GetPayload() != nil {
				t.Errorf("payload = %T, want none", env.GetPayload())
			}
			if env.GetId() != "5d2b7c19-e8a4-4f03-a6b1-2c9d8e7f6a50" || env.GetProducer().GetType() != "future-collector" {
				t.Errorf("known fields not decoded: %v", env)
			}
			if len(env.ProtoReflect().GetUnknown()) == 0 {
				t.Error("the unknown payload is not kept as unknown fields")
			}
		},
	},
}

// encode gives a fixture's files: name -> content.
func encode(t *testing.T, f fixture) map[string][]byte {
	t.Helper()
	if f.raw != nil {
		return map[string][]byte{f.name + ".binpb": f.raw(t)}
	}
	bin, err := proto.MarshalOptions{Deterministic: true}.Marshal(f.msg)
	if err != nil {
		t.Fatal(err)
	}
	js, err := protojson.Marshal(f.msg)
	if err != nil {
		t.Fatal(err)
	}
	// protojson output is deliberately unstable: indent it ourselves.
	var out bytes.Buffer
	if err := json.Indent(&out, js, "", "  "); err != nil {
		t.Fatal(err)
	}
	out.WriteByte('\n')
	return map[string][]byte{f.name + ".binpb": bin, f.name + ".json": out.Bytes()}
}

func TestUpdate(t *testing.T) {
	if !*update {
		t.Skip("run with -update to write the files of new fixtures")
	}
	for _, f := range all {
		for name, content := range encode(t, f) {
			p := filepath.Join("v1", name)
			if _, err := os.Stat(p); err == nil {
				continue // append-only: never rewritten
			}
			if err := os.WriteFile(p, content, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", p)
		}
	}
}

// TestFixtures decodes every file of FS and compares it with its definition.
func TestFixtures(t *testing.T) {
	want := map[string]bool{}
	for _, f := range all {
		for name := range encode(t, f) {
			want[name] = true
		}
		t.Run(f.name, func(t *testing.T) {
			bin, err := fs.ReadFile(fixtures.FS, "v1/"+f.name+".binpb")
			if err != nil {
				t.Fatalf("%v (run go test ./fixtures -update)", err)
			}
			var got airtgv1.Envelope
			if err := proto.Unmarshal(bin, &got); err != nil {
				t.Fatalf("binary: %v", err)
			}
			if f.raw != nil {
				f.check(t, &got)
				return
			}
			if !proto.Equal(&got, f.msg) {
				t.Errorf("binary file differs from the definition:\n got %v\nwant %v", &got, f.msg)
			}
			js, err := fs.ReadFile(fixtures.FS, "v1/"+f.name+".json")
			if err != nil {
				t.Fatal(err)
			}
			got.Reset()
			if err := protojson.Unmarshal(js, &got); err != nil {
				t.Fatalf("json: %v", err)
			}
			if !proto.Equal(&got, f.msg) {
				t.Errorf("JSON file differs from the definition:\n got %v\nwant %v", &got, f.msg)
			}
		})
	}
	// No file without a definition.
	entries, err := fs.ReadDir(fixtures.FS, "v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !want[e.Name()] {
			t.Errorf("v1/%s has no definition in fixtures_test.go", e.Name())
		}
	}
}

// TestContract checks the fixtures follow the rules of the schema's
// comments, so they are examples worth copying.
func TestContract(t *testing.T) {
	for _, f := range all {
		env := f.msg
		if env == nil {
			continue
		}
		if env.GetId() == "" || env.GetSchemaVersion() == "" || env.GetProducer().GetType() == "" || env.GetTime() == nil {
			t.Errorf("%s: envelope header incomplete", f.name)
		}
		var dev *airtgv1.Device
		switch p := env.GetPayload().(type) {
		case *airtgv1.Envelope_Metrics:
			dev = p.Metrics.GetDevice()
			if p.Metrics.GetInterval().AsDuration() <= 0 {
				t.Errorf("%s: no interval", f.name)
			}
			for _, s := range p.Metrics.GetSamples() {
				if s.GetMode() == airtgv1.Mode_MODE_UNSPECIFIED || s.GetValue() == nil || s.GetTime() == nil {
					t.Errorf("%s: incomplete sample %v", f.name, s)
				}
				if (s.GetTable() == "") != (s.GetIndex() == "") {
					t.Errorf("%s: table and index must be both set or both empty: %v", f.name, s)
				}
				if s.GetMode() != airtgv1.Mode_MODE_RAW {
					if _, ok := s.GetValue().(*airtgv1.Sample_DoubleValue); !ok {
						t.Errorf("%s: rate/volume must be a double: %v", f.name, s)
					}
					if s.GetMode() == airtgv1.Mode_MODE_RATE && s.GetUnit() != "" && !strings.HasSuffix(s.GetUnit(), "/s") {
						t.Errorf("%s: rate unit without /s: %v", f.name, s)
					}
				}
			}
		case *airtgv1.Envelope_Inventory:
			dev = p.Inventory.GetDevice()
			if !slices.IsSorted(p.Inventory.GetCapabilities()) {
				t.Errorf("%s: capabilities not sorted", f.name)
			}
		default:
			t.Errorf("%s: no payload", f.name)
		}
		if !schema.ValidID(dev.GetId()) {
			t.Errorf("%s: invalid device id %q", f.name, dev.GetId())
		}
		for _, id := range []string{env.GetTenantId(), env.GetProducer().GetId()} {
			if id != "" && !schema.ValidID(id) {
				t.Errorf("%s: invalid tenant or producer id %q", f.name, id)
			}
		}
	}
}

// Example of a consumer test reading the fixtures.
func ExampleFS() {
	bin, _ := fs.ReadFile(fixtures.FS, path.Join("v1", "metrics-dotted-id.binpb"))
	var env airtgv1.Envelope
	if err := proto.Unmarshal(bin, &env); err != nil {
		panic(err)
	}
	id := env.GetMetrics().GetDevice().GetId()
	fmt.Println(id, schema.MetricsSubject(id))
	// Output: 10.0.0.1-161 airtg.v1.metrics.10~2e0~2e0~2e1-161
}
