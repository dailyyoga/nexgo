package kafka

import (
	"sort"
	"testing"
)

// realisticStatsJSON is a trimmed but structurally faithful librdkafka
// statistics document. It includes two topics, the internal "-1" aggregate
// partition, a real partition whose lag is not yet known (consumer_lag -1) and
// real partitions with valid lag.
const realisticStatsJSON = `{
  "name": "rdkafka#consumer-1",
  "type": "consumer",
  "topics": {
    "raw-events": {
      "topic": "raw-events",
      "partitions": {
        "-1": {"partition": -1, "consumer_lag": -1},
        "0":  {"partition": 0,  "consumer_lag": 12},
        "1":  {"partition": 1,  "consumer_lag": 0},
        "2":  {"partition": 2,  "consumer_lag": -1}
      }
    },
    "events": {
      "topic": "events",
      "partitions": {
        "0": {"partition": 0, "consumer_lag": 5}
      }
    }
  }
}`

func TestParseStatsLag(t *testing.T) {
	got := parseStatsLag(realisticStatsJSON)

	// Stable order for comparison.
	sort.Slice(got, func(i, j int) bool {
		if got[i].Topic != got[j].Topic {
			return got[i].Topic < got[j].Topic
		}
		return got[i].Partition < got[j].Partition
	})

	want := []statsLag{
		{Topic: "events", Partition: 0, Lag: 5},
		{Topic: "raw-events", Partition: 0, Lag: 12},
		{Topic: "raw-events", Partition: 1, Lag: 0},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d entries %+v, want %d %+v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseStatsLagDefensive(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty string", ""},
		{"invalid json", "{not json"},
		{"no topics key", `{"name":"x","type":"consumer"}`},
		{"null topics", `{"topics": null}`},
		{"empty topics", `{"topics": {}}`},
		{"missing partitions", `{"topics": {"t": {"topic": "t"}}}`},
		{"wrong type for lag", `{"topics": {"t": {"partitions": {"0": {"consumer_lag": "oops"}}}}}`},
		{"non-numeric partition", `{"topics": {"t": {"partitions": {"abc": {"consumer_lag": 3}}}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic; returns no usable entries.
			if got := parseStatsLag(tc.in); len(got) != 0 {
				t.Errorf("expected no entries, got %+v", got)
			}
		})
	}
}
