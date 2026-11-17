package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"iter"
)

// maxJSONLLine bounds one line [ReadJSONL] accepts. [Filesystem] caps every
// string field at 1024 characters, so a record it wrote is far smaller.
const maxJSONLLine = 1 << 20

// ReadJSONL yields the records of a JSONL stream as [Filesystem] writes
// it, one per non-empty line. A line that does not decode yields an error
// naming its line number, and the sequence stops.
func ReadJSONL(r io.Reader) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), maxJSONLLine)
		line := 0
		for sc.Scan() {
			line++
			if len(sc.Bytes()) == 0 {
				continue
			}
			var rec Record
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				yield(Record{}, fmt.Errorf("line %d: %w", line, err))
				return
			}
			if !yield(rec, nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield(Record{}, fmt.Errorf("line %d: %w", line+1, err))
		}
	}
}
