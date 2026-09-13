package main

import (
	"errors"
	"time"
)

const (
	saveQuitStepSave         = "saveToFile"
	saveQuitStepGracefulQuit = "gracefulQuit"
	saveQuitStepDelayedEnter = "delayedEnter"
	saveQuitStepPkill        = "pkill"
	saveQuitStepNeedUser     = "needUser"
)

// errStudioDidNotQuit is a dead end: Studio is still up after save, Enter,
// graceful quit, and pkill. Callers print NEED_USER.
var errStudioDidNotQuit = errors.New("Roblox Studio did not quit")

type saveQuitStep struct {
	Name     string
	Wait     time.Duration
	StopIfOK bool
}

// saveQuitPlan: save, quit, Enter if a Save dialog is up, wait for that
// save, quit again, then pkill so the agent is not stuck. NEED_USER only
// if force-quit also fails.
func saveQuitPlan() []saveQuitStep {
	return []saveQuitStep{
		{Name: saveQuitStepSave},
		{Name: saveQuitStepGracefulQuit, Wait: 3 * time.Second, StopIfOK: true},
		{Name: saveQuitStepDelayedEnter},
		{Name: saveQuitStepGracefulQuit, Wait: 12 * time.Second, StopIfOK: true},
		{Name: saveQuitStepPkill, Wait: 8 * time.Second, StopIfOK: true},
		{Name: saveQuitStepNeedUser},
	}
}

type saveQuitHooks struct {
	Save    func() error
	Quit    func(wait time.Duration) error // Apple Event + wait; never pkill
	Enter   func() error
	Kill    func(wait time.Duration) error // pkill last resort, after Enter
	Running func() bool
}

func runSaveQuitPlan(h saveQuitHooks) error {
	running := func() bool {
		if h.Running == nil {
			return true
		}
		return h.Running()
	}
	stopIfGone := func(err error, step saveQuitStep) bool {
		return step.StopIfOK && (err == nil || !running())
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
			if stopIfGone(err, step) {
				return nil
			}
		case saveQuitStepDelayedEnter:
			if h.Enter != nil {
				_ = h.Enter()
			}
		case saveQuitStepPkill:
			var err error
			if h.Kill != nil {
				err = h.Kill(step.Wait)
			}
			if stopIfGone(err, step) {
				return nil
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
