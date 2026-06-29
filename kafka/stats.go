package kafka

import (
	"encoding/json"
	"strconv"
)

// statsLag is one extracted per-partition consumer lag entry.
type statsLag struct {
	Topic     string
	Partition int32
	Lag       int64
}

// parseStatsLag extracts per-partition consumer lag from a librdkafka statistics
// JSON document (the string emitted by *kafka.Stats). It is intentionally
// defensive: any structural surprise (invalid JSON, missing keys, wrong types)
// yields fewer entries rather than a panic, because the document follows
// librdkafka's internal schema which we do not control.
//
// Two kinds of entries are skipped:
//   - partition "-1": librdkafka's internal "unassigned" aggregate, not a real
//     partition.
//   - negative lag: librdkafka reports consumer_lag = -1 when the lag is not yet
//     known; emitting that as a gauge would be misleading.
func parseStatsLag(statsJSON string) []statsLag {
	var root struct {
		Topics map[string]struct {
			Partitions map[string]struct {
				ConsumerLag int64 `json:"consumer_lag"`
			} `json:"partitions"`
		} `json:"topics"`
	}
	if err := json.Unmarshal([]byte(statsJSON), &root); err != nil {
		return nil
	}

	var out []statsLag
	for topic, t := range root.Topics {
		for partStr, p := range t.Partitions {
			partition, err := strconv.Atoi(partStr)
			if err != nil || partition < 0 {
				continue
			}
			if p.ConsumerLag < 0 {
				continue
			}
			out = append(out, statsLag{
				Topic:     topic,
				Partition: int32(partition),
				Lag:       p.ConsumerLag,
			})
		}
	}
	return out
}
