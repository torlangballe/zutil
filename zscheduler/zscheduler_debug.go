package zscheduler

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zint"
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

	Count        int
	JobName      string
	ExecutorName string
}

var (
	DebugLog                      = zlog.NewEnabler()
	debugPrintExecutorRowsPrinted int
)

func (s *Scheduler[I]) setDebugState(jobID I, existing, starting, ending, running bool) {
	now := time.Now()
	d, got := s.Debug.Get(jobID)
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
		d.Count++
		d.starting = now
	} else if running {
		d.running = now
	} else if ending {
		d.ending = now
	}
	if !starting && !running && !ending {
		d.ExecutorName = ""
	}
	s.Debug.Set(jobID, d)
}

func intPadded(i int) string {
	if i == 0 {
		return "   "
	}
	return fmt.Sprintf("%-3d", i)
}

func (s *Scheduler[I]) DebugPrintExecutors(run Run[I], sit SituationType) {
	// if s == JobEnded {
	// 	zlog.Warn("DebugPrintExecutors", run.Job.ID, s)
	// }
	runningCount := map[I]int{}
	startingCount := map[I]int{}
	endingCount := map[I]int{}
	for _, r := range s.runs {
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
	exes := append([]Executor[I]{{DebugName: "Wrk0"}}, s.executors...)
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
	switch sit {
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

func debugRow(row JobDebug, w io.Writer) {
	kn := time.Since(row.known)
	ex := addedTime(row.Existed, row.existing)
	st := addedTime(row.Started, row.starting)
	en := addedTime(row.Ended, row.ending)
	ru := addedTime(row.Runned, row.running)
	fmt.Fprint(w, row.JobName, "\t", row.ExecutorName, "\t", kn, "\t", ex, "\t", st, "\t", en, "\t", ru, "\t", kn-ex-st-en-ru, "\t", row.Count, "\n")
}

func (s *Scheduler[I]) PrintDebugRows(w io.Writer) {
	var st, et time.Duration
	if s.Debug.Count() == 0 {
		fmt.Fprintln(w, "No Debug Job Rows")
		return
	}
	tabWriter := zstr.NewTabWriter(w)
	fmt.Fprint(tabWriter, zstr.EscGreen, "job\texcutor\tknown\tunused\tstarting\tending\trun\tgone\tcount", zstr.EscNoColor, "\n")
	s.Debug.ForEach(func(key I, row JobDebug) bool {
		st += row.Started
		et += row.Ended
		debugRow(row, tabWriter)
		return true
	})
	tabWriter.Flush()
}

func idToInt64[I comparable](i I) int64 {
	switch v := any(i).(type) {
	case int:
		return int64(v)
	case int64:
		return v
	default:
		return zint.HashTo64(fmt.Sprint(i))
	}
}

func (s *Scheduler[I]) CommandNodes(cs *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	nodes := zcommands.NodesForStruct(cs, s, wild, zcommands.FieldNode, forExpand)

	runs := zcommands.MakeNode("runs", zcommands.ComNode, &RunsCom[I]{s: s}, 0)
	nodes = append(nodes, runs)

	executors := zcommands.MakeNode("executors", zcommands.ComNode, &ExecutorsCom[I]{s: s}, 0)
	nodes = append(nodes, executors)

	setup := zcommands.MakeNode("setup", zcommands.ComNode, &s.setup, 0)
	nodes = append(nodes, setup)
	return nodes
}

type RunsCom[I comparable] struct {
	s *Scheduler[I]
}

type ExecutorsCom[I comparable] struct {
	s *Scheduler[I]
}

func (rc *RunsCom[I]) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	var nodes []zcommands.Node
	for i := range rc.s.runs {
		run := rc.s.runs[i]
		n := zcommands.MakeNode(run.Job.DebugName, zcommands.RowNode|zcommands.ComNode, &run, idToInt64(run.Job.ID))
		nodes = append(nodes, n)
	}
	return nodes
}

func (run *Run[I]) CommandColumns() zdict.Items {
	return zdict.MakeItems(
		".id", idToInt64(run.Job.ID),
		"name", run.Job.DebugName,
		"executor", run.ExecutorID,
		"started@", run.StartedAt,
		"ran@", run.RanAt,
		"stopped@", run.StoppedAt,
		"milestone@", run.MilestoneAt,
		"progress", run.Progress(),
		"removing", run.Removing,
		"duration", run.Job.Duration,
	)
}

func (ec *ExecutorsCom[I]) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	var nodes []zcommands.Node
	for i := range ec.s.executors {
		executor := ec.s.executors[i]
		n := zcommands.MakeNode(executor.DebugName, zcommands.RowNode|zcommands.ComNode, &executor, idToInt64(executor.ID))
		nodes = append(nodes, n)
	}
	return nodes
}

func (e *Executor[I]) CommandColumns() zdict.Items {
	return zdict.MakeItems(
		".id", idToInt64(e.ID),
		"name", e.DebugName,
		"able", e.IsAble,
		"paused", e.Paused,
		"alive@", e.KeptAliveAt,
		"cost", e.CostCapacity,
		"attributes", e.AcceptAttributes,
	)
}
