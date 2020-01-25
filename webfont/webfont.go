// Package webfont performs common rune and glyph processing operations.
package webfont

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode/utf8"
)

// Processor is an interface used to process glyphs.
type Processor interface {
	// ProcessGlyph is called when the glyph has been fully parsed.
	ProcessGlyph(g *Glyph)
	// The following operations can be used to record SVG font paths with lossless detail.
	MoveTo(x, y float64)
	LineTo(x, y float64)
	CubicTo(x1, y1, x2, y2, ex, ey float64)
	QuadraticTo(x1, y1, x2, y2 float64)
}

// ParseNeededGlyphs parses the needed glyphs from the font and
// populates the glyph details. If processor is non-nil, it calls
// the processor methods as it processes each necessary glyph.
func ParseNeededGlyphs(fontData *FontData, message string, processor Processor) error {
	if fontData == nil {
		return errors.New("fontData must not be nil")
	}

	glyphLess := func(a, b int) bool {
		sa, sb := "", ""
		if fontData.Font.Glyphs[a].Unicode != nil {
			sa = *fontData.Font.Glyphs[a].Unicode
		}
		if fontData.Font.Glyphs[b].Unicode != nil {
			sb = *fontData.Font.Glyphs[b].Unicode
		}
		return strings.Compare(sa, sb) < 0
	}

	sort.Slice(fontData.Font.Glyphs, glyphLess)

	// Fix UTF8 rune errors and de-duplicate identical code points.
	dedup := map[rune]*Glyph{}
	var dst rune = 0xfbf0
	for _, g := range fontData.Font.Glyphs {
		if g.Unicode == nil {
			continue
		}
		r := UTF8toRune(g.Unicode)
		if r == 0 {
			return fmt.Errorf("unicode %+q maps to r=0", *g.Unicode)
		}
		if message != "" && !strings.ContainsRune(message, r) {
			continue
		}
		if _, ok := dedup[r]; ok {
			if dst == 0xfeff { // BOM - disallowed in Go source.
				dst++
			}
			for {
				if _, ok := dedup[dst]; !ok {
					break
				}
				dst++
			}
			log.Printf("WARNING: unicode %+q found multiple times in font. Moving code point to %+q", r, dst)
			rs := fmt.Sprintf("%c", dst)
			g.Unicode = &rs
			dedup[dst] = g
			dst++
			continue
		}
		rs := fmt.Sprintf("%c", r)
		g.Unicode = &rs
		dedup[r] = g
	}

	// re-sort with deduped glyph code points.
	sort.Slice(fontData.Font.Glyphs, glyphLess)

	for _, g := range fontData.Font.Glyphs {
		r := UTF8toRune(g.Unicode)
		if g.Unicode == nil || (message != "" && !strings.ContainsRune(message, r)) {
			continue
		}
		g.ParsePath()
		g.GenGerberLP(fontData.Font.FontFace)
		if g.MBB.Area() == 0.0 {
			continue
		}

		if processor != nil {
			processor.ProcessGlyph(g)
		}
	}

	return nil
}

// UTF8toRune converts the utf8 codepoint to a rune.
func UTF8toRune(s *string) rune {
	if s == nil || *s == "" {
		return 0
	}

	switch *s {
	case "\n":
		return '\n'
	case `\`:
		return '\\'
	case `'`:
		return '\''
	}

	if utf8.RuneCountInString(*s) == 1 {
		r, _ := utf8.DecodeRuneInString(*s)
		return r
	}
	if r, ok := specialCase[*s]; ok {
		return r
	}

	if len(*s) > 1 {
		log.Printf("WARNING: Unhandled unicode seqence: %+q", *s)
	}
	for _, r := range *s { // Return the first rune
		return r
	}
	return 0
}
