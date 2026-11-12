package schema

import (
	"slices"
	"strings"
)

// Units is the unit vocabulary of Sample.unit: every collector names the
// units of its values with these words, so that the same unit is always
// written the same way ("percent", never "%" or "pct"). Each is the source's
// own unit, lowercase singular: values are never scaled to a common unit,
// so a kilobyte value stays in "kilobyte". A rate appends "/s" to the unit
// of its counter ("octet/s"), and so does a source value that is a rate
// itself (ifSpeed, in "bit/s").
//
// A unit missing here is added in a new minor version of the schema, never
// invented by a collector. "centisecond" is absent on purpose: SNMP time
// ticks are converted to "second", the unit everyone uses for time.
var Units = []string{
	// Data.
	"bit", "kilobit", "megabit", "gigabit",
	"byte", "kilobyte", "megabyte", "gigabyte", "terabyte",
	"octet", "allocation unit",
	// Network.
	"packet", "frame", "segment", "datagram",
	// Time.
	"nanosecond", "microsecond", "millisecond", "second", "minute", "hour", "day",
	// Ratio.
	"percent",
	// Things counted.
	"connection", "entry", "error", "event", "message", "process", "request", "route",
	"session", "thread", "user",
	// Physical.
	"celsius", "fahrenheit", "kelvin",
	"milliwatt", "watt", "kilowatt", "watt-hour", "kilowatt-hour", "volt-ampere",
	"millivolt", "volt", "milliampere", "ampere",
	"hertz", "kilohertz", "megahertz", "gigahertz",
	"rpm", "decibel", "dbm",
	// A number that names a state or a thing rather than measuring a
	// quantity: ifOperStatus (1 up, 2 down, ...), a process id.
	"enum",
}

// ValidUnit reports whether u is a unit of the vocabulary, alone or as a
// rate ("<unit>/s").
func ValidUnit(u string) bool {
	return slices.Contains(Units, strings.TrimSuffix(u, "/s"))
}
