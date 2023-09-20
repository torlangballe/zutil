package zprocess

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

// This file contains code for debugging collected information for jobs, or
// outputting information about each executor's state on any change.
// Neither are used directly in unit tests, but might be helpful.

// JobDebug stores the duration each job is starting, running and ending. It also stores
// how long it has existed, i.e has also been in the scheduler. Known remembers when it
// was first added to the scheduler, even after removed.
type JobDebug struct {
	known    time.Time
	existing time.Time
	starting time.Time
	ending   time.Time
	running  time.Time

	Existed time.Duration
	Started time.Duration
	Ended   time.Duration
	Runned  time.Duration

	JobName      string
	ExecutorName string
}

var (
	debugLog                      zlog.Enabler
	debugPrintExecutorRowsPrinted int
)

func (b *Scheduler[I]) setDebugState(jobID I, existing, starting, ending, running bool) {
	now := time.Now()
	d, got := b.Debug.Get(jobID)
	if !got {
		d.known = now
	}
	if !d.existing.IsZero() {
		d.Existed += time.Since(d.existing)
		d.existing = time.Time{}
	}
	if !d.starting.IsZero() {
		d.Started += time.Since(d.starting)
		d.starting = time.Time{}
	}
	if !d.ending.IsZero() {
		d.Ended += time.Since(d.ending)
		d.ending = time.Time{}
	}
	if !d.running.IsZero() {
		d.Runned += time.Since(d.running)
		d.running = time.Time{}
	}
	// zlog.Warn("setDebugState:", d.JobName, jobID, existing, starting, running, delta, str)
	if existing {
		d.existing = now
		d.ExecutorName = ""
	} else if starting {
		d.starting = now
	} else if running {
		d.running = now
	} else if ending {
		d.ending = now
	}
	if !starting && !running && !ending {
		d.ExecutorName = ""
	}
	b.Debug.Set(jobID, d)
}

func intPadded(i int) string {
	if i == 0 {
		return "   "
	}
	return fmt.Sprintf("%-3d", i)
}

func (b *Scheduler[I]) DebugPrintExecutors(run Run[I], s SituationType) {
	runningCount := map[I]int{}
	startingCount := map[I]int{}
	endingCount := map[I]int{}
	for _, r := range b.runs {
		if r.Job.ID == run.Job.ID {
			r = run
		}
		if !r.RanAt.IsZero() {
			runningCount[r.ExecutorID]++
		} else if !r.StoppedAt.IsZero() {
			endingCount[r.ExecutorID]++
		} else if !r.StartedAt.IsZero() {
			startingCount[r.ExecutorID]++
		}
	}
	// zlog.Warn("Ending++", endingCount)
	exes := append([]Executor[I]{{DebugName: "Wrk0"}}, b.executors...)
	sort.Slice(exes, func(i, j int) bool {
		return strings.Compare(exes[i].DebugName, exes[j].DebugName) > 0
	})
	if debugPrintExecutorRowsPrinted%20 == 0 {
		fmt.Printf("                    ")
		for _, e := range exes {
			fmt.Printf("%-9s  ", e.DebugName)
		}
		fmt.Println(zstr.EscNoColor)
	}
	var str string
	switch s {
	case JobStarted:
		str = "S"
	case JobEnded:
		str = "E"
	case JobRunning:
		str = "r"
	case JobStopped:
		str = "s"
	}
	var mid string
	for _, e := range exes {
		mid += zstr.EscYellow + intPadded(startingCount[e.ID])
		mid += zstr.EscGreen + intPadded(runningCount[e.ID])
		mid += zstr.EscRed + intPadded(endingCount[e.ID])
		mid += "  "
	}
	zlog.Warn(fmt.Sprintf("%s%-2v@%v  ", str, run.Job.ID, run.ExecutorID), mid, zstr.EscNoColor)
	debugPrintExecutorRowsPrinted++
}

func addedTime(d time.Duration, t time.Time) time.Duration {
	if !t.IsZero() {
		d += time.Since(t)
	}
	return d
}

func debugRow(row JobDebug) {
	kn := time.Since(row.known)
	ex := addedTime(row.Existed, row.existing)
	st := addedTime(row.Started, row.starting)
	en := addedTime(row.Ended, row.ending)
	ru := addedTime(row.Runned, row.running)
	zlog.Warn(row.JobName, row.ExecutorName, "known:", kn, "existed:", ex, "starting:", st, "ending:", en, "run:", ru, "gone:", kn-ex-st-en-ru)
}

func (b *Scheduler[I]) PrintDebugRows() {
	var st, et time.Duration
	b.Debug.ForEach(func(key I, row JobDebug) bool {
		st += row.Started
		et += row.Ended
		debugRow(row)
		return true
	})
	fmt.Println()
}
