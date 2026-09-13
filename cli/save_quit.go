package main

import (
	"errors"
	"time"
)

const (
	saveQuitStepSave         = "saveToFile"
	saveQuitStepGracefulQuit = "gracefulQuit"
	saveQuitStepDelayedEnter = "delayedEnter"
	saveQuitStepNeedUser     = "needUser"
)

// errStudioDidNotQuit means a Save / Don't Save / Cancel dialog is likely
// still up. Callers print NEED_USER. This path never pkill's Studio.
var errStudioDidNotQuit = errors.New("Roblox Studio did not quit")

type saveQuitStep struct {
	Name     string
	Wait     time.Duration
	StopIfOK bool
}

// saveQuitPlan is the restart-quit policy: save, graceful quit, Enter if
// still running, graceful quit again, then NEED_USER. pkill is not a step.
func saveQuitPlan() []saveQuitStep {
	return []saveQuitStep{
		{Name: saveQuitStepSave},
		{Name: saveQuitStepGracefulQuit, Wait: 3 * time.Second, StopIfOK: true},
		{Name: saveQuitStepDelayedEnter},
		{Name: saveQuitStepGracefulQuit, Wait: 12 * time.Second, StopIfOK: true},
		{Name: saveQuitStepNeedUser},
	}
}

type saveQuitHooks struct {
	Save    func() error
	Quit    func(wait time.Duration) error // Apple Event + wait; never pkill
	Enter   func() error
	Running func() bool
}

func runSaveQuitPlan(h saveQuitHooks) error {
	running := func() bool {
		if h.Running == nil {
			return true
		}
		return h.Running()
	}
	for _, step := range saveQuitPlan() {
		switch step.Name {
		case saveQuitStepSave:
			if h.Save != nil {
				_ = h.Save()
			}
		case saveQuitStepGracefulQuit:
			var err error
			if h.Quit != nil {
				err = h.Quit(step.Wait)
			}
			if step.StopIfOK && (err == nil || !running()) {
				return nil
			}
		case saveQuitStepDelayedEnter:
			if h.Enter != nil {
				_ = h.Enter()
			}
		case saveQuitStepNeedUser:
			if !running() {
				return nil
			}
			return errStudioDidNotQuit
		default:
			return errStudioDidNotQuit
		}
	}
	return errStudioDidNotQuit
}
