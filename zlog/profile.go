package zlog

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
)

type Profile struct {
	Name    string
	Start   time.Time
	Lines   []Line
	MinSecs float64
}

type average struct {
	MinDur   float64
	MaxDur   float64
	SumDur   float64
	DurCount int

	AllMinDur   float64
	AllMaxDur   float64
	AllSumDur   float64
	AllDurCount int
}

type Line struct {
	Text     string
	Duration time.Duration
	Time     time.Time
}

var (
	averages     = map[string]*average{}
	averagesLock sync.Mutex
)

func PrintProfileAverages(printSecs float64) {
	go repeatPrintAverages(printSecs)
}

func (a *average) SetDuration(dur float64) {
	zfloat.Minimize(&a.MinDur, dur)
	zfloat.Maximize(&a.MaxDur, dur)
	if a.DurCount == 0 {
		a.MinDur = dur
	}
	a.SumDur += dur
	a.DurCount++

	zfloat.Minimize(&a.AllMinDur, dur)
	zfloat.Maximize(&a.AllMaxDur, dur)
	if a.AllDurCount == 0 {
		a.AllMinDur = dur
	}
	a.AllSumDur += dur
	a.AllDurCount++
}

func NewProfile(minSecs float64, name ...any) *Profile {
	var p Profile
	p.Name = zstr.Spaced(name...)
	p.Start = time.Now()
	p.MinSecs = minSecs
	pre := getAverageName(p.Name)
	averagesLock.Lock()
	_, got := averages[pre]
	if !got {
		averages[pre] = &average{}
	}
	averagesLock.Unlock()
	return &p
}

func getAverageName(name string) string {
	return strings.TrimSpace(zstr.HeadUntil(name, ":"))
}

func (p *Profile) Log(parts ...any) {
	var line *Line
	previ := -1
	str := zstr.Spaced(parts...)
	for i, l := range p.Lines {
		if l.Text == str {
			line = &p.Lines[i]
			previ = i - 1
			break
		}
	}
	if line == nil {
		previ = len(p.Lines) - 1
		p.Lines = append(p.Lines, Line{})
		line = &p.Lines[previ+1]
		line.Time = time.Now()
		line.Text = str
	}
	start := p.Start
	if previ > -1 {
		start = p.Lines[previ].Time
	}
	line.Duration += time.Since(start)
	// Info("Add:", p.Name, line.Duration)
}

func (p *Profile) End(parts ...any) {
	if len(parts) != 0 {
		since := time.Since(p.Start)
		if since > time.Duration(p.MinSecs) {
			p.Log(zstr.Spaced(parts), since)
		}
	}
	dur := time.Since(p.Start)
	var print bool
	for _, l := range p.Lines {
		if l.Duration > time.Duration(p.MinSecs*float64(time.Second)) {
			print = true
			break
		}
	}
	pre := getAverageName(p.Name)
	averagesLock.Lock()
	a, _ := averages[pre]
	Assert(a != nil, pre)
	a.SetDuration(float64(dur) / float64(time.Second))
	averagesLock.Unlock()
	if !print {
		return
	}
	// Info("zprofile", p.Name, "dump:", p.Start)
	for _, l := range p.Lines {
		percent := int(float64(l.Duration) / float64(dur) * 100)
		Info("zprofile", p.Name+":", l.Text, "    ", l.Duration, percent, `%`)
	}
}

func valueStr(v float64, valid bool) string {
	if !valid {
		return ""
	}
	if v < 0.5 {
		return fmt.Sprintf("%d", int(v*1000))
	}
	f := zfloat.KeepFractionDigits(v, 2)
	return zstr.EscMagenta + fmt.Sprintf("%gs", f) + zstr.EscNoColor
}

func repeatPrintAverages(printSecs float64) {
	for {
		var tab *zstr.TabWriter
		time.Sleep(time.Duration(float64(time.Second) * printSecs))
		averagesLock.Lock()
		names := zmap.KeysAsStrings(averages)
		sort.Strings(names)
		for _, n := range names {
			a := averages[n]
			if a.AllDurCount == 0 {
				continue
			}
			if tab == nil {
				tab = zstr.NewTabWriter(os.Stdout)
				fmt.Fprintln(tab, zstr.EscYellow+"zlog.Profile.name\tmin\tmax\taverage\tcount"+zstr.EscNoColor)
			}
			valid := (a.DurCount != 0)
			min := valueStr(a.MinDur, valid)
			max := valueStr(a.MaxDur, valid)
			count := strconv.Itoa(a.DurCount)
			if a.DurCount == 0 {
				count = ""
			}
			avg := valueStr(a.SumDur/float64(a.DurCount), valid)
			amin := valueStr(a.AllMinDur, true)
			amax := valueStr(a.AllMaxDur, true)
			aavg := valueStr(a.AllSumDur/float64(a.AllDurCount), true)
			acount := strconv.Itoa(a.AllDurCount)
			fmt.Fprintf(tab, "%s\t%s/%s\t%s/%s\t%s/%s\t%s/%s\n", n, min, amin, max, amax, avg, aavg, count, acount)
			a.MaxDur = 0
			a.DurCount = 0
			a.SumDur = 0
		}
		averagesLock.Unlock()
		if tab != nil {
			tab.Flush()
		}
	}
}
