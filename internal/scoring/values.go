package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/umbralux1/entropic/internal/signal"
)

// Values holds one value per Signal. A missing Signal reads as zero.
type Values map[signal.Signal]int

// Weighted is the Values' contribution to the Entropy Score, before
// normalising by size, with each Signal weighed by weights.
func (v Values) Weighted(weights map[signal.Signal]float64) float64 {
	total := 0.0
	for _, s := range signal.OfCode {
		total += weights[s] * float64(v[s])
	}
	return total
}

func (v Values) add(other Values) Values {
	out := Values{}
	for _, s := range signal.OfCode {
		out[s] = v[s] + other[s]
	}
	return out
}

func (v Values) minus(other Values) Values {
	out := Values{}
	for _, s := range signal.OfCode {
		out[s] = v[s] - other[s]
	}
	return out
}

func (v Values) isZero() bool {
	for _, s := range signal.OfCode {
		if v[s] != 0 {
			return false
		}
	}
	return true
}

// marshalWith writes obj, a struct without custom marshalling, as a JSON
// object followed by one field per Signal, so that Signal values sit beside
// the other fields.
func marshalWith(obj any, v Values) ([]byte, error) {
	fields, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(bytes.TrimSuffix(fields, []byte("}")))
	for _, s := range signal.OfCode {
		fmt.Fprintf(&b, ",%q:%d", s, v[s])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// MarshalJSON writes the Unit with one field per Signal.
func (u Unit) MarshalJSON() ([]byte, error) {
	type plain Unit
	return marshalWith(plain(u), u.Values)
}

// MarshalJSON writes the Delta with one field per Signal.
func (d UnitDelta) MarshalJSON() ([]byte, error) {
	type plain UnitDelta
	return marshalWith(plain(d), d.Values)
}
