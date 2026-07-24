package docs

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func rankDocuments(documents []document, query string, topK int, filters Filters, opts Options) []Hit {
	var hits []Hit
	for _, doc := range documents {
		if filters.Language != "" && doc.Language != filters.Language {
			continue
		}
		if filters.Domain != "" && doc.Domain != filters.Domain {
			continue
		}
		chunks := doc.Chunks
		if len(chunks) == 0 {
			chunks = []chunk{{ID: "doc", Heading: doc.Title, Text: doc.Text}}
		}
		for _, ch := range chunks {
			score := keywordScore(query, doc.Path, ch.Text, []string{ch.Heading}, opts)
			if score < opts.MinScore {
				continue
			}
			hits = append(hits, Hit{
				Path:     doc.Path,
				Title:    doc.Title,
				Heading:  ch.Heading,
				ChunkID:  ch.ID,
				Language: doc.Language,
				Domain:   doc.Domain,
				AppID:    doc.AppID,
				Excerpt:  snippet(ch.Text, query, 500),
				Score:    score,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	if topK < 1 {
		topK = 1
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

func keywordScore(query, path, text string, headings []string, opts Options) float64 {
	q := strings.ToLower(strings.TrimSpace(query))
	words := meaningfulQueryWords(q)
	if len(words) == 0 {
		return 0
	}
	textLower := strings.ToLower(text)
	pathLower := strings.ToLower(path)

	score := 0.0
	matched := 0
	for _, word := range words {
		inText := strings.Count(textLower, word)
		inPath := strings.Contains(pathLower, word)
		if inText == 0 && !inPath && opts.fuzzyEnabled() {
			if closestToken(word, textLower+" "+pathLower) != "" {
				matched++
				score += 0.75
			}
		}
		if inText > 0 || inPath {
			matched++
		}
		score += float64(inText) * 1.0
		if inPath {
			score += 5.0
		}
	}

	need := 2
	if len(words) < need {
		need = len(words)
	}
	if matched < need {
		return 0
	}

	if strings.Contains(textLower, q) {
		score += 12.0
	}

	for _, heading := range headings {
		headingLower := strings.ToLower(heading)
		for _, word := range words {
			if strings.Contains(headingLower, word) {
				score += 3.0
			}
		}
	}
	return score
}

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "buat": {}, "cara": {}, "di": {}, "do": {}, "for": {}, "how": {},
	"in": {}, "ke": {}, "of": {}, "on": {}, "the": {}, "to": {}, "untuk": {}, "use": {}, "yang": {},
}

func meaningfulQueryWords(query string) []string {
	raw := splitWords(query)
	seen := map[string]struct{}{}
	var out []string
	for _, w := range raw {
		w = strings.ToLower(w)
		if utf8.RuneCountInString(w) < 3 {
			continue
		}
		if _, stop := stopWords[w]; stop {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}

func splitWords(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func closestToken(needle, haystack string) string {
	tokens := splitWords(haystack)
	maxDistance := 1
	if utf8.RuneCountInString(needle) >= 7 {
		maxDistance = 2
	}
	best := ""
	bestDistance := maxDistance + 1
	seen := map[string]struct{}{}
	for _, token := range tokens {
		token = strings.ToLower(token)
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		if abs(utf8.RuneCountInString(token)-utf8.RuneCountInString(needle)) > maxDistance {
			continue
		}
		d := levenshtein(needle, token)
		if d <= maxDistance && d < bestDistance {
			best = token
			bestDistance = d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := 0; j <= len(rb); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min3(del, ins, sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func snippet(text, query string, maxLen int) string {
	lower := strings.ToLower(text)
	qLower := strings.ToLower(query)
	pos := strings.Index(lower, qLower)
	if pos < 0 {
		for _, word := range strings.Fields(qLower) {
			pos = strings.Index(lower, word)
			if pos >= 0 {
				break
			}
		}
	}
	if pos < 0 {
		return truncateRunes(text, maxLen)
	}
	runes := []rune(text)
	rpos := utf8.RuneCountInString(lower[:min(pos, len(lower))])
	if rpos > len(runes) {
		rpos = 0
	}
	rstart := rpos - 80
	if rstart < 0 {
		rstart = 0
	}
	end := rstart + maxLen
	if end > len(runes) {
		end = len(runes)
	}
	return strings.TrimSpace(string(runes[rstart:end]))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
