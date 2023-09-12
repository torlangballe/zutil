package zprocess

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type JobDebug struct {
	Known    time.Time
	Existing time.Time
	Starting time.Time
	Ending   time.Time
	Running  time.Time

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
		d.Known = now
	}
	if !d.Existing.IsZero() {
		d.Existed += time.Since(d.Existing)
		d.Existing = time.Time{}
	}
	if !d.Starting.IsZero() {
		d.Started += time.Since(d.Starting)
		d.Starting = time.Time{}
	}
	if !d.Ending.IsZero() {
		d.Ended += time.Since(d.Ending)
		d.Ending = time.Time{}
	}
	if !d.Running.IsZero() {
		d.Runned += time.Since(d.Running)
		d.Running = time.Time{}
	}
	// zlog.Warn("setDebugState:", d.JobName, jobID, existing, starting, running, delta, str)
	if existing {
		d.Existing = now
		d.ExecutorName = ""
	} else if starting {
		d.Starting = now
	} else if running {
		d.Running = now
	} else if ending {
		d.Ending = now
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
	fmt.Printf("%s: %s%-2v@%v  ", time.Now().Format("15:04:05.09"), str, run.Job.ID, run.ExecutorID)
	for _, e := range exes {
		fmt.Printf(zstr.EscYellow + intPadded(startingCount[e.ID]))
		fmt.Printf(zstr.EscGreen + intPadded(runningCount[e.ID]))
		fmt.Printf(zstr.EscRed + intPadded(endingCount[e.ID]))
		fmt.Print("  ")
	}
	fmt.Println(zstr.EscNoColor)
	debugPrintExecutorRowsPrinted++
}
