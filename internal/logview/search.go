package logview

import "regexp"

type searchResult struct {
	// 0-indexed line number where match appears. Negative for a match in
	// the buffer.
	line int

	// 0-indexed column number of the rune that begins the match.
	char int

	// Length of the match.
	length int
}

func (m *Model) search(query string) ([]searchResult, error) {
	var results []searchResult
	re, err := regexp.Compile(query)
	if err != nil {
		return nil, err
	}
	for i, l := range m.lines {
		for _, m := range re.FindAllStringIndex(l, -1) {
			results = append(results, searchResult{
				line:   i,
				char:   m[0],
				length: m[1] - m[0],
			})
		}
	}
	if m.buffer != "" {
		for _, m := range re.FindAllStringIndex(m.buffer, -1) {
			results = append(results, searchResult{
				line:   -1,
				char:   m[0],
				length: m[1] - m[0],
			})
		}
	}
	return results, nil
}
