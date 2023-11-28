package zlog

import (
	"time"

	"github.com/torlangballe/zutil/zstr"
)

type Profile struct {
	Name    string
	Start   time.Time
	Lines   []Line
	MinSecs float64
}

type Line struct {
	Text     string
	Duration time.Duration
	Time     time.Time
}

func NewProfile(name string, minSecs float64) Profile {
	p := Profile{}
	p.Name = name
	p.Start = time.Now()
	p.MinSecs = minSecs
	return p
}

func (p *Profile) Log(parts ...interface{}) {
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

func (p *Profile) End(parts ...interface{}) {
	if len(parts) != 0 {
		p.Log(parts...)
	}
	dur := time.Since(p.Start)
	var print bool
	for _, l := range p.Lines {
		if l.Duration > time.Duration(p.MinSecs*float64(time.Second)) {
			print = true
			break
		}
	}
	if !print {
		return
	}
	Info("zprofile", p.Name, "dump:", p.Start)
	for _, l := range p.Lines {
		percent := int(float64(l.Duration) / float64(dur) * 100)
		Info("zprofile", p.Name+":", l.Text, "    ", l.Duration, percent, `%`)
	}
}
