package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSaveQuitPlanOrder(t *testing.T) {
	plan := saveQuitPlan()
	var names []string
	var quitWaits []time.Duration
	var pkillWait time.Duration
	enterAt, pkillAt := -1, -1
	for i, s := range plan {
		names = append(names, s.Name)
		switch s.Name {
		case saveQuitStepGracefulQuit:
			quitWaits = append(quitWaits, s.Wait)
			if !s.StopIfOK {
				t.Fatal("graceful quit must stop the plan if Studio actually exited")
			}
		case saveQuitStepDelayedEnter:
			enterAt = i
		case saveQuitStepPkill:
			pkillAt = i
			pkillWait = s.Wait
			if !s.StopIfOK {
				t.Fatal("pkill must stop the plan if Studio exited")
			}
		}
	}
	want := []string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
		saveQuitStepPkill,
		saveQuitStepNeedUser,
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("plan %v want %v", names, want)
	}
	if enterAt < 0 || pkillAt < 0 || pkillAt <= enterAt {
		t.Fatalf("pkill must run after Enter so a Save dialog can flush: enter=%d pkill=%d", enterAt, pkillAt)
	}
	if len(quitWaits) != 2 {
		t.Fatalf("expected two graceful quits, waits=%v", quitWaits)
	}
	if quitWaits[0] <= 0 || quitWaits[0] > 5*time.Second {
		t.Fatalf("first graceful quit must be short so Enter can run: %s", quitWaits[0])
	}
	if quitWaits[1] <= quitWaits[0] {
		t.Fatalf("second graceful quit should wait longer (time to save): %v", quitWaits)
	}
	if pkillWait <= 0 {
		t.Fatal("pkill must wait for Studio to die")
	}
}

func TestRunSaveQuitPlanStopsAfterFirstQuit(t *testing.T) {
	var calls []string
	err := runSaveQuitPlan(saveQuitHooks{
		Save: func() error {
			calls = append(calls, saveQuitStepSave)
			return nil
		},
		Quit: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepGracefulQuit)
			if wait != saveQuitPlan()[1].Wait {
				t.Fatalf("first quit wait %s want %s", wait, saveQuitPlan()[1].Wait)
			}
			return nil
		},
		Enter: func() error {
			calls = append(calls, saveQuitStepDelayedEnter)
			return nil
		},
		Kill: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepPkill)
			return nil
		},
		Running: func() bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, ",")
	if got != saveQuitStepSave+","+saveQuitStepGracefulQuit {
		t.Fatalf("Enter/pkill must not run after a successful graceful quit; got %s", got)
	}
}

func TestRunSaveQuitPlanEnterThenSecondQuit(t *testing.T) {
	var calls []string
	var waits []time.Duration
	n := 0
	err := runSaveQuitPlan(saveQuitHooks{
		Save: func() error {
			calls = append(calls, saveQuitStepSave)
			return nil
		},
		Quit: func(wait time.Duration) error {
			n++
			waits = append(waits, wait)
			calls = append(calls, saveQuitStepGracefulQuit)
			if n == 1 {
				return fmt.Errorf("dialog up")
			}
			return nil
		},
		Enter: func() error {
			if n != 1 {
				t.Fatal("Enter must run after the first failed graceful quit, before pkill")
			}
			calls = append(calls, saveQuitStepDelayedEnter)
			return nil
		},
		Kill: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepPkill)
			t.Fatal("pkill must not run if the second graceful quit succeeded")
			return nil
		},
		Running: func() bool { return n < 2 },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, ",")
	want := strings.Join([]string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
	}, ",")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	plan := saveQuitPlan()
	if len(waits) != 2 || waits[0] != plan[1].Wait || waits[1] != plan[3].Wait {
		t.Fatalf("quit waits %v", waits)
	}
}

func TestRunSaveQuitPlanPkillAfterEnter(t *testing.T) {
	var calls []string
	var killWait time.Duration
	err := runSaveQuitPlan(saveQuitHooks{
		Save: func() error {
			calls = append(calls, saveQuitStepSave)
			return nil
		},
		Quit: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepGracefulQuit)
			return fmt.Errorf("dialog up")
		},
		Enter: func() error {
			calls = append(calls, saveQuitStepDelayedEnter)
			return nil
		},
		Kill: func(wait time.Duration) error {
			if !strings.Contains(strings.Join(calls, ","), saveQuitStepDelayedEnter) {
				t.Fatal("pkill must not run before Enter")
			}
			killWait = wait
			calls = append(calls, saveQuitStepPkill)
			return nil
		},
		Running: func() bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, ",")
	want := strings.Join([]string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
		saveQuitStepPkill,
	}, ",")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	plan := saveQuitPlan()
	if killWait != plan[4].Wait {
		t.Fatalf("pkill wait %s want %s", killWait, plan[4].Wait)
	}
}

func TestRunSaveQuitPlanNeedUserAfterPkillFails(t *testing.T) {
	var calls []string
	err := runSaveQuitPlan(saveQuitHooks{
		Save: func() error {
			calls = append(calls, saveQuitStepSave)
			return nil
		},
		Quit: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepGracefulQuit)
			return fmt.Errorf("still running")
		},
		Enter: func() error {
			calls = append(calls, saveQuitStepDelayedEnter)
			return nil
		},
		Kill: func(wait time.Duration) error {
			calls = append(calls, saveQuitStepPkill)
			return fmt.Errorf("pkill failed")
		},
		Running: func() bool { return true },
	})
	if !errors.Is(err, errStudioDidNotQuit) {
		t.Fatalf("want errStudioDidNotQuit after pkill failed, got %v", err)
	}
	got := strings.Join(calls, ",")
	want := strings.Join([]string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
		saveQuitStepPkill,
	}, ",")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
