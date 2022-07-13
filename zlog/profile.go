package zlog

import (
	"time"

	"github.com/torlangballe/zutil/zstr"
)

type Profile struct {
	Name  string
	Start time.Time
	Lines []Line
}

type Line struct {
	Text     string
	Duration time.Duration
	Time     time.Time
}

type PeridocLogger struct {
	last         time.Time
	durationSecs float64
}

var profiles []Profile

func PushProfile(name string) {
	p := Profile{}
	p.Name = name
	p.Start = time.Now()
	profiles = append(profiles, p)
}

func ProfileLog(parts ...interface{}) {
	var line *Line
	previ := -1
	p := &profiles[len(profiles)-1]
	str := zstr.SprintSpaced(parts...)
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

func EndProfile(parts ...interface{}) {
	if len(parts) != 0 {
		ProfileLog(parts...)
	}
	p := &profiles[len(profiles)-1]
	dur := time.Since(p.Start)
	for _, l := range p.Lines {
		percent := int(float64(l.Duration) / float64(dur) * 100)
		Info(p.Name+":", l.Text, time.Since(p.Start), "    ", l.Duration, percent, "%")
	}
	RemoveProfile()
}

func RemoveProfile() {
	profiles = profiles[:len(profiles)-1]
}

