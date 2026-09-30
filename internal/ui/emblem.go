package ui

import "strings"

// The ResoNET emblem from the web brand (assets/icons/icon-512.png): three
// concentric steel rings with crosshair ticks, split by a vertical beam that
// runs blue at the top, magenta through the middle and orange at the bottom.
// Each art row is a left half plus the beam column; the right half is the
// left half mirrored, so the rings stay symmetric by construction.

type emblemRow struct {
	left string
	beam string
}

var emblemLarge = []emblemRow{
	{"                    ", "│"},
	{"         ,──────────", "┼"},
	{"     ,──'   ,───────", "┼"},
	{"   ,'    ,─'  ,─────", "┼"},
	{"  /    ,'   ,'      ", "│"},
	{" │    /    /        ", "("},
	{"─┼─  │    │         ", ")"},
	{" │    \\    \\        ", "("},
	{"  \\    ',   ',      ", "│"},
	{"   ',    '─,  '─────", "┼"},
	{"     '──,   '───────", "┼"},
	{"         '──────────", "┼"},
	{"                    ", "│"},
}

var emblemSmall = []emblemRow{
	{"       ", "│"},
	{"  ,────", "┼"},
	{" / ,───", "┼"},
	{"─┼ │   ", ")"},
	{" \\ '───", "┼"},
	{"  '────", "┼"},
	{"       ", "│"},
}

var emblemMirror = strings.NewReplacer("/", "\\", "\\", "/", "(", ")", ")", "(")

func mirrorEmblemHalf(left string) string {
	runes := []rune(left)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return emblemMirror.Replace(string(runes))
}

// emblemBeamColor walks the beam from blue down through magenta to orange.
func emblemBeamColor(row, rows int) string {
	switch {
	case row*3 < rows:
		return Bold + FgBlue
	case row*3 < rows*2:
		return Bold + FgMagenta
	default:
		return FgOrange
	}
}

func welcomeEmblemArt(width int) []string {
	rows := emblemLarge
	if normalizeScreenWidth(width) < 72 {
		rows = emblemSmall
	}
	artWidth := len([]rune(rows[0].left))*2 + 1
	indent := (normalizeScreenWidth(width) - 2 - artWidth) / 2
	if indent < 1 {
		indent = 1
	}
	pad := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		beam := emblemBeamColor(i, len(rows))
		lines = append(lines, pad+
			FgWhite+row.left+Reset+
			beam+row.beam+Reset+
			FgWhite+mirrorEmblemHalf(row.left)+Reset)
	}
	return lines
}
