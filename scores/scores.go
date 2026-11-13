// Package scores reads paired measurement records.
//
// A record is one outcome of one system on one item:
//
//	{"split":"hold","system":"policy-v7","item":"task-042","score":1}
//
// Files may be JSON Lines or a single JSON array; the format is detected from
// the first non-space character. Scores are numbers in [0,1]: a binary
// outcome is 0 or 1, a graded outcome may sit between.
//
// Records are keyed by split, system and item, so a baseline and a candidate
// can be paired item by item. An item present for one system and missing for
// the other is dropped rather than silently imputed.
package scores

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Record is one measured outcome.
type Record struct {
	Split  string  `json:"split"`
	System string  `json:"system"`
	Item   string  `json:"item"`
	Score  float64 `json:"score"`
}

// Dataset holds every record grouped by split, system and item.
type Dataset struct {
	// Splits maps a split label to its systems.
	Splits map[string]map[string]map[string]float64
}

// NewDataset returns an empty dataset.
func NewDataset() *Dataset {
	return &Dataset{Splits: map[string]map[string]map[string]float64{}}
}

// Add records one outcome.
func (d *Dataset) Add(r Record) error {
	if strings.TrimSpace(r.Split) == "" {
		return fmt.Errorf("record has no split")
	}
	if strings.TrimSpace(r.System) == "" {
		return fmt.Errorf("record has no system")
	}
	if strings.TrimSpace(r.Item) == "" {
		return fmt.Errorf("record has no item")
	}
	if r.Score < 0 || r.Score > 1 {
		return fmt.Errorf("record %s/%s/%s has score %v outside [0,1]", r.Split, r.System, r.Item, r.Score)
	}
	bySystem, ok := d.Splits[r.Split]
	if !ok {
		bySystem = map[string]map[string]float64{}
		d.Splits[r.Split] = bySystem
	}
	byItem, ok := bySystem[r.System]
	if !ok {
		byItem = map[string]float64{}
		bySystem[r.System] = byItem
	}
	if _, exists := byItem[r.Item]; exists {
		return fmt.Errorf("duplicate record for %s/%s/%s", r.Split, r.System, r.Item)
	}
	byItem[r.Item] = r.Score
	return nil
}

// Systems returns the system names present in a split, sorted.
func (d *Dataset) Systems(split string) []string {
	bySystem, ok := d.Splits[split]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(bySystem))
	for name := range bySystem {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Pair returns the per-item difference candidate minus baseline for the items
// both systems were measured on, together with the item names.
//
// The order of the returned slice follows the sorted item names, so a pairing
// is reproducible from the data alone.
func (d *Dataset) Pair(split, baseline, candidate string) ([]float64, []string, error) {
	bySystem, ok := d.Splits[split]
	if !ok {
		return nil, nil, fmt.Errorf("no data for split %q", split)
	}
	baseItems, ok := bySystem[baseline]
	if !ok {
		return nil, nil, fmt.Errorf("no data for system %q in split %q", baseline, split)
	}
	candItems, ok := bySystem[candidate]
	if !ok {
		return nil, nil, fmt.Errorf("no data for system %q in split %q", candidate, split)
	}
	shared := make([]string, 0, len(baseItems))
	for item := range baseItems {
		if _, ok := candItems[item]; ok {
			shared = append(shared, item)
		}
	}
	sort.Strings(shared)
	diffs := make([]float64, 0, len(shared))
	for _, item := range shared {
		diffs = append(diffs, candItems[item]-baseItems[item])
	}
	return diffs, shared, nil
}

// Load reads records from a file.
func Load(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadFrom(f)
}

// LoadFrom reads records from a reader.
func LoadFrom(r io.Reader) (*Dataset, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimLeft(string(raw), " \t\r\n")
	if trimmed == "" {
		return nil, fmt.Errorf("no records found")
	}
	d := NewDataset()
	if trimmed[0] == '[' {
		var records []Record
		if err := json.Unmarshal([]byte(trimmed), &records); err != nil {
			return nil, fmt.Errorf("invalid JSON array: %w", err)
		}
		for i, rec := range records {
			if err := d.Add(rec); err != nil {
				return nil, fmt.Errorf("record %d: %w", i, err)
			}
		}
		return d, nil
	}
	line := 0
	for _, l := range strings.Split(trimmed, "\n") {
		line++
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", line, err)
		}
		if err := d.Add(rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
	}
	return d, nil
}
