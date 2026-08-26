package adapters

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
)

// ForEachJSONLLimited walks a JSONL file without loading it all: the first
// headBytes and, if the file is larger, the last tailBytes (partial first line
// of the tail is dropped). Malformed lines are skipped.
func ForEachJSONLLimited(path string, headBytes, tailBytes int64, fn func(map[string]interface{})) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size == 0 {
		return nil
	}
	if tailBytes < 0 {
		tailBytes = 0
	}
	if headBytes < 0 {
		headBytes = 0
	}
	if size <= headBytes+tailBytes {
		return scanJSONL(f, fn)
	}

	if err := scanJSONL(io.LimitReader(f, headBytes), fn); err != nil {
		return err
	}
	if _, err := f.Seek(size-tailBytes, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReader(f)
	if _, err := r.ReadBytes('\n'); err != nil && err != io.EOF {
		return err
	}
	return scanJSONL(r, fn)
}

func scanJSONL(r io.Reader, fn func(map[string]interface{})) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec map[string]interface{}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		fn(rec)
	}
	return sc.Err()
}
