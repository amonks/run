package logview

type searchResult struct {
	// 0-indexed line number where match appears. Negative for a match in
	// the buffer.
	line int

	// 0-indexed column number of the rune that begins the match.
	char int

	// Length of the match.
	length int
}

func (m *Model) search() []searchResult {
	if m.queryRe == nil {
		return nil
	}

	var results []searchResult
	for i := range m.lines {
		results = append(results, m.searchLine(i)...)
	}
	if m.buffer != "" {
		results = append(results, m.searchLine(-1)...)
	}
	return results
}

func (m *Model) searchLine(lineno int) []searchResult {
	if m.queryRe == nil {
		return nil
	}

	var line string
	if lineno < 0 {
		line = m.buffer
	} else {
		line = m.lines[lineno]
	}
	var results []searchResult
	for _, m := range m.queryRe.FindAllStringIndex(line, -1) {
		results = append(results, searchResult{
			line:   lineno,
			char:   m[0],
			length: m[1] - m[0],
		})
	}
	return results
}
