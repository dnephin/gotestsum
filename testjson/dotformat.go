package testjson

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"gotest.tools/gotestsum/internal/dotwriter"
)

func dotsFormatV1(event TestEvent, exec *Execution) (string, error) {
	pkg := exec.Package(event.Package)
	switch {
	case event.PackageEvent():
		return "", nil
	case event.Action == ActionRun && pkg.Total == 1:
		return "[" + RelativePackagePath(event.Package) + "]", nil
	}
	return fmtDot(event), nil
}

func fmtDot(event TestEvent) string {
	withColor := colorEvent(event)
	switch event.Action {
	case ActionPass:
		return withColor("·")
	case ActionFail:
		return withColor("✖")
	case ActionSkip:
		return withColor("↷")
	}
	return ""
}

type dotFormatter struct {
	pkgs      map[string]*dotLine
	order     []string
	writer    *dotwriter.Writer
	termWidth int
}

type dotLine struct {
	runes      int
	builder    *strings.Builder
	lastUpdate time.Time
	full       bool
}

func (l *dotLine) update(dot string) {
	if l.full || dot == "" {
		return
	}
	l.builder.WriteString(dot)
	l.runes++
}

// checkWidth marks the line as full when the width of the line hits the
// terminal width.
func (l *dotLine) checkWidth(prefix, terminal int) {
	// padding is the space required for the carriage return added when the line
	// is full.
	const padding = 1
	if prefix+l.runes+padding >= terminal && !l.full {
		l.builder.WriteString("↲")
		l.full = true
	}
}

func newDotFormatter(out io.Writer) EventFormatter {
	w, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
	if w == 0 {
		logrus.Warn("Failed to detect terminal width for dots format.")
		return &formatAdapter{format: dotsFormatV1, out: out}
	}
	return &dotFormatter{
		pkgs:      make(map[string]*dotLine),
		writer:    dotwriter.New(out),
		termWidth: w,
	}
}

func (d *dotFormatter) Format(event TestEvent, exec *Execution) error {
	if d.pkgs[event.Package] == nil {
		d.pkgs[event.Package] = &dotLine{builder: new(strings.Builder)}
		d.order = append(d.order, event.Package)
	}
	line := d.pkgs[event.Package]
	line.lastUpdate = event.Time

	if !event.PackageEvent() {
		line.update(fmtDot(event))
	}
	switch event.Action {
	case ActionOutput, ActionBench:
		return nil
	}

	sort.Slice(d.order, d.orderByLastUpdated)
	for _, pkg := range d.order {
		line := d.pkgs[pkg]
		prefix, width := fmtDotPkgTime(RelativePackagePath(pkg), exec.Package(pkg))
		line.checkWidth(width, d.termWidth)
		fmt.Fprintf(d.writer, prefix+line.builder.String()+"\n")
	}
	return d.writer.Flush()
}

// orderByLastUpdated so that the most recently updated packages move to the
// bottom of the list, leaving completed package in the same order at the top.
func (d *dotFormatter) orderByLastUpdated(i, j int) bool {
	return d.pkgs[d.order[i]].lastUpdate.Before(d.pkgs[d.order[j]].lastUpdate)
}

// TODO: test case for timing format
func fmtDotPkgTime(pkg string, p *Package) (string, int) {
	elapsed := p.Elapsed()
	var pkgTime string
	switch {
	case p.cached:
		pkgTime = "🖴"
	case elapsed == 0:
	case elapsed < time.Second:
		pkgTime = elapsed.String()
	case elapsed < 10*time.Second:
		pkgTime = elapsed.Truncate(time.Millisecond).String()
	case elapsed < time.Minute:
		pkgTime = elapsed.Truncate(time.Second).String()
	}

	// fixed is the width of the fixed size prefix, plus 2 spaces for padding.
	const fixed = 8
	return fmt.Sprintf("%6s %s ", pkgTime, pkg), len(pkg) + fixed
}