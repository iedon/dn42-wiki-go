package site

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	searchIndexVersion = 3
	maxPositionsPerDoc = 48
)

var (
	searchIndexFields    = []string{"title", "summary", "content"}
	emptySearchIndexJSON = json.RawMessage(`{"v":3,"c":0,"f":["title","summary","content"],"a":[0,0,0],"d":[],"t":{}}`)
)

type termEntry struct {
	DocID       int
	TitleFreq   int
	SummaryFreq int
	ContentFreq int
	Positions   []int
}

func buildSearchIndex(pages []page) (json.RawMessage, error) {
	if len(pages) == 0 {
		return append(json.RawMessage(nil), emptySearchIndexJSON...), nil
	}

	docs := make([][]string, 0, len(pages))
	termMap := make(map[string][]*termEntry, len(pages)*16)
	var sumLengths [3]int

	for docID, pg := range pages {
		docTerms := make(map[string]*termEntry, 64)

		titleLen := processField(pg.Title, func(token string) {
			entry := docTerms[token]
			if entry == nil {
				entry = &termEntry{DocID: docID}
				docTerms[token] = entry
			}
			entry.TitleFreq++
		})

		summaryLen := processField(pg.Summary, func(token string) {
			entry := docTerms[token]
			if entry == nil {
				entry = &termEntry{DocID: docID}
				docTerms[token] = entry
			}
			entry.SummaryFreq++
		})

		contentPos := 0
		contentLen := processField(pg.PlainText, func(token string) {
			entry := docTerms[token]
			if entry == nil {
				entry = &termEntry{DocID: docID}
				docTerms[token] = entry
			}
			entry.ContentFreq++
			if len(entry.Positions) < maxPositionsPerDoc {
				entry.Positions = append(entry.Positions, contentPos)
			}
			contentPos++
		})

		sumLengths[0] += titleLen
		sumLengths[1] += summaryLen
		sumLengths[2] += contentLen

		meta := encodeLengths(titleLen, summaryLen, contentLen)
		docs = append(docs, []string{pg.Route, pg.Title, pg.Summary, meta})

		for term, entry := range docTerms {
			termMap[term] = append(termMap[term], entry)
		}
	}

	termStrings := make(map[string]string, len(termMap))
	termKeys := make([]string, 0, len(termMap))
	for term := range termMap {
		termKeys = append(termKeys, term)
	}
	sort.Strings(termKeys)

	for _, term := range termKeys {
		entries := termMap[term]
		sort.Slice(entries, func(i, j int) bool { return entries[i].DocID < entries[j].DocID })
		termStrings[term] = encodeTermEntries(entries)
	}

	avgLengths := make([]int, len(sumLengths))
	docCount := len(pages)
	for i := range sumLengths {
		if docCount == 0 {
			avgLengths[i] = 0
			continue
		}
		avgLengths[i] = int(math.Round(float64(sumLengths[i]*100) / float64(docCount)))
	}

	payload := struct {
		Version         int               `json:"v"`
		DocCount        int               `json:"c"`
		Fields          []string          `json:"f"`
		AvgFieldLengths []int             `json:"a"`
		Docs            [][]string        `json:"d"`
		Terms           map[string]string `json:"t"`
	}{
		Version:         searchIndexVersion,
		DocCount:        docCount,
		Fields:          append([]string(nil), searchIndexFields...),
		AvgFieldLengths: avgLengths,
		Docs:            docs,
		Terms:           termStrings,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func processField(text string, apply func(string)) int {
	if text == "" {
		return 0
	}
	normalized := norm.NFKD.String(text)
	var builder strings.Builder
	count := 0
	for _, r := range normalized {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
		default:
			if builder.Len() > 0 {
				token := builder.String()
				builder.Reset()
				if shouldIndexToken(token) {
					apply(token)
					count++
				}
			}
		}
	}
	if builder.Len() > 0 {
		token := builder.String()
		builder.Reset()
		if shouldIndexToken(token) {
			apply(token)
			count++
		}
	}
	return count
}

func shouldIndexToken(token string) bool {
	if token == "" {
		return false
	}
	if len(token) == 1 {
		b := token[0]
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func encodeTermEntries(entries []*termEntry) string {
	var builder strings.Builder
	builder.Grow(len(entries) * 12)
	builder.WriteString(encodeInt(len(entries)))
	builder.WriteByte('|')
	for i, entry := range entries {
		if i > 0 {
			builder.WriteByte(';')
		}
		builder.WriteString(encodeInt(entry.DocID))
		builder.WriteByte(':')
		builder.WriteString(encodeInt(entry.TitleFreq))
		builder.WriteByte(':')
		builder.WriteString(encodeInt(entry.SummaryFreq))
		builder.WriteByte(':')
		builder.WriteString(encodeInt(entry.ContentFreq))
		if len(entry.Positions) > 0 {
			builder.WriteByte(':')
			builder.WriteString(encodePositions(entry.Positions))
		}
	}
	return builder.String()
}

func encodeLengths(titleLen, summaryLen, contentLen int) string {
	return encodeInt(titleLen) + "," + encodeInt(summaryLen) + "," + encodeInt(contentLen)
}

func encodeInt(value int) string {
	if value == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, 0, 8)
	for value > 0 {
		remainder := value % 36
		buf = append([]byte{digits[remainder]}, buf...)
		value /= 36
	}
	return string(buf)
}

func encodePositions(positions []int) string {
	if len(positions) == 0 {
		return ""
	}
	var builder strings.Builder
	prev := 0
	for i, pos := range positions {
		diff := pos
		if i > 0 {
			diff = pos - prev
		}
		prev = pos
		if i > 0 {
			builder.WriteByte('.')
		}
		builder.WriteString(encodeInt(diff))
	}
	return builder.String()
}
