package protocol

import (
	"encoding/json"
	"fmt"
)

// mergeExtra marshals base then merges extra keys into the resulting object.
func mergeExtra(base any, extra map[string]RawPayload) ([]byte, error) {
	if len(extra) == 0 {
		return json.Marshal(base)
	}
	b, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	for k, v := range extra {
		if _, exists := m[k]; exists {
			continue // base fields win
		}
		m[k] = json.RawMessage(v)
	}
	return json.Marshal(m)
}

// splitExtra unmarshals data into dest and returns unknown object keys as Extra.
// known lists JSON keys that belong to dest (so they are not put in Extra).
func splitExtra(data []byte, dest any, known ...string) (map[string]RawPayload, error) {
	if err := json.Unmarshal(data, dest); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	knownSet := make(map[string]struct{}, len(known))
	for _, k := range known {
		knownSet[k] = struct{}{}
	}
	var extra map[string]RawPayload
	for k, v := range raw {
		if _, ok := knownSet[k]; ok {
			continue
		}
		if extra == nil {
			extra = make(map[string]RawPayload)
		}
		extra[k] = RawPayload(v)
	}
	return extra, nil
}

// errf is fmt.Errorf with a short name for AcceptHello and friends.
func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

