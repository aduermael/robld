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
	for _, s := range plan {
		names = append(names, s.Name)
		if strings.Contains(strings.ToLower(s.Name), "pkill") {
			t.Fatal("save/quit plan must not pkill while a Save dialog may be up")
		}
		if s.Name == saveQuitStepGracefulQuit {
			quitWaits = append(quitWaits, s.Wait)
			if !s.StopIfOK {
				t.Fatal("graceful quit must stop the plan if Studio actually exited")
			}
		}
	}
	want := []string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
		saveQuitStepNeedUser,
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("plan %v want %v", names, want)
	}
	if len(quitWaits) != 2 {
		t.Fatalf("expected two graceful quits, waits=%v", quitWaits)
	}
	if quitWaits[0] <= 0 || quitWaits[0] > 5*time.Second {
		t.Fatalf("first graceful quit must be short so Enter can run: %s", quitWaits[0])
	}
	if quitWaits[1] <= quitWaits[0] {
		t.Fatalf("second graceful quit should wait longer than the first: %v", quitWaits)
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
		Running: func() bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, ",")
	if got != saveQuitStepSave+","+saveQuitStepGracefulQuit {
		t.Fatalf("Enter must not run after a successful graceful quit; got %s", got)
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

func TestRunSaveQuitPlanNeedUserWhenStillRunning(t *testing.T) {
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
		Running: func() bool { return true },
	})
	if !errors.Is(err, errStudioDidNotQuit) {
		t.Fatalf("want errStudioDidNotQuit, got %v", err)
	}
	got := strings.Join(calls, ",")
	want := strings.Join([]string{
		saveQuitStepSave,
		saveQuitStepGracefulQuit,
		saveQuitStepDelayedEnter,
		saveQuitStepGracefulQuit,
	}, ",")
	if got != want {
		t.Fatalf("got %s want %s (pkill must not run)", got, want)
	}
	for _, c := range calls {
		if strings.Contains(strings.ToLower(c), "pkill") {
			t.Fatal("pkill must not be on the save-dialog path")
		}
	}
}
