package fsm

import (
	"context"
	"fmt"
	"time"
)

type StateEventTuple struct {
	state StateType
	event EventType
}

func Tuple(state StateType, event EventType) StateEventTuple {
	return StateEventTuple{
		event: event,
		state: state,
	}
}

type Transitions map[StateEventTuple]StateType
type CallbackFn func(context.Context, *State, *EventData)
type Callbacks map[StateType]CallbackFn

type Fsm struct {
	transitions   Transitions
	callbacks     Callbacks
	commonEvents  map[EventType]bool
	commonHandler CallbackFn
	done          chan struct{}
	w             Executer
	metrics       FsmMetrics
}

type Options struct {
	Transitions    Transitions
	Callbacks      Callbacks
	CommonCallback CallbackFn
	CommonEvents   []EventType
}

func NewFsm(opts Options, w Executer) *Fsm {
	ret := &Fsm{
		transitions:   make(map[StateEventTuple]StateType),
		callbacks:     make(map[StateType]CallbackFn),
		commonEvents:  make(map[EventType]bool),
		commonHandler: opts.CommonCallback,
		done:          make(chan struct{}),
		w:             w,
		metrics:       newFsmMetrics(),
	}

	for s, fn := range opts.Callbacks {
		ret.callbacks[s] = fn
	}

	knownStates := make(map[StateType]bool)
	knownEvents := make(map[EventType]bool)
	for t, s := range opts.Transitions {
		knownStates[t.state] = true
		knownEvents[t.event] = true
		ret.transitions[t] = s
	}

	for s, _ := range knownStates {
		if _, ok := opts.Callbacks[s]; !ok {
			panic("unknown state in callback map")
		}
	}

	// set a common handler and a list of non-transitional events that will be
	// handled by the handler
	for _, ev := range opts.CommonEvents {
		if _, ok := knownEvents[ev]; ok {
			panic("Common event must not in the transision list")
		} else {
			ret.commonEvents[ev] = true
		}
	}

	//go ret.loop()
	return ret
}

type Executer interface {
	Go(func()) error
}

// Send an event, return a chanel to receive an error reporting if the event is
// invalid on current state
// A caller should never try to retrieve the error if it is within another
// callback. Recursive calling will cause a race condition.
func (fsm *Fsm) SendEvent(state *State, event *EventData) chan error {
	errCh := make(chan error, 1)
	fsm.handleEvent(state, event, errCh, false)
	return errCh
}

// Send an event and wait for it to complete then return error indicating if the
// event was handle.
func (fsm *Fsm) SyncSendEvent(state *State, event *EventData) error {
	errCh := make(chan error, 1)
	fsm.handleEvent(state, event, errCh, true)
	return <-errCh
}

func (fsm *Fsm) processNextEvent(state *State) {
	for state.nextEv != nil {
		t := time.Now()
		fsm.metrics.onSubmitted()
		fsm.metrics.onTriggered()
		nextEv := state.nextEv
		state.nextEv = nil //reset next event for the state
		if _, ok := fsm.commonEvents[nextEv.Type()]; ok {
			fsm.executeCallback(fsm.commonHandler, state, nextEv)
		} else { //if it is a transitional event
			fsm.transit(state, nextEv, nil)
		}
		fsm.metrics.onCompleted(nextEv.Type(), t)
	}
}

func (fsm *Fsm) executeCallback(callback CallbackFn, state *State, event *EventData) {
	if callback == nil {
		return
	}
	state.nextEvSetter = fsm.setNextEventSetter(state.current, event.Type()) //set setter
	callback(event.ctx, state, event)                                        //execute callback
	state.nextEvSetter = nil                                                 //reset setter
}

func (fsm *Fsm) setNextEventSetter(current StateType, evType EventType) func(*State, *EventData) {
	return func(state *State, ev *EventData) {
		if evType == ExitEvent {
			panic("SetNextEvent in an ExitEvent callback is not allowed")
			return
		}
		if fsm.isTransited(current, evType) {
			panic("SetNextEvent right after a transit is not allowed")
			return
		}
		if state.nextEv != nil {
			panic("Multiple SetNextEvent is called")
			return
		}
		state.nextEv = ev
	}
}

func (fsm *Fsm) isTransited(current StateType, evType EventType) bool {
	if _, ok := fsm.commonEvents[evType]; !ok { //not a common event
		if nextState, ok := fsm.transitions[StateEventTuple{ //must have a next state
			state: current,
			event: evType,
		}]; ok {
			return nextState != current //and next state is different  from current state
		}
	}
	return false
}

func (fsm *Fsm) handleEvent(state *State, event *EventData, errCh chan error, sync bool) {
	//a state only process one event at a time, so we need to lock it
	//release the state lock after finish handling the event
	fsm.metrics.onSubmitted()

	var fn func()
	var isCommon bool

	//if the event is in the list of common events
	if _, isCommon = fsm.commonEvents[event.Type()]; isCommon {
		fn = func() {
			state.evLock.Lock()
			t := time.Now()
			fsm.metrics.onTriggered()
			fsm.executeCallback(fsm.commonHandler, state, event)
			fsm.metrics.onCompleted(event.Type(), t)
			fsm.processNextEvent(state)
			state.evLock.Unlock()
		}
	} else { //if it is a transitional event
		fn = func() {
			state.evLock.Lock()
			t := time.Now()
			fsm.metrics.onTriggered()
			fsm.transit(state, event, errCh)
			fsm.metrics.onCompleted(event.Type(), t)
			fsm.processNextEvent(state)
			state.evLock.Unlock() //unlock the state
		}
	}
	if sync {
		fn()
	} else { //handle the event in a worker pool
		if err := fsm.w.Go(fn); err != nil { //fail to send to a workerpool
			errCh <- err
			return
		}
	}
	if isCommon {
		errCh <- nil
	}
}

func (fsm *Fsm) transit(state *State, event *EventData, errCh chan error) {
	current := state.CurrentState()

	if nextState, ok := fsm.transitions[StateEventTuple{
		state: current,
		event: event.Type(),
	}]; ok { //have a next state
		if errCh != nil {
			errCh <- nil
		}
		curCallback := fsm.callbacks[current]
		nextCallback := fsm.callbacks[nextState]

		//execute callback for the event
		fsm.executeCallback(curCallback, state, event)

		if current != nextState { //state will be changed
			//exectute callback for ExitEvent of the current state
			fsm.executeCallback(curCallback, state, event.clone(ExitEvent))

			//change to the next state
			state.setState(nextState)

			//execute callback for EtryEvent of the next state
			fsm.executeCallback(nextCallback, state, event.clone(EntryEvent))

		}
	} else {
		if errCh != nil {
			errCh <- fmt.Errorf("Unknown transition from state %v with event %v", current, event)
		}
	}
}

func (fsm *Fsm) Info() *FsmInfo {
	return fsm.metrics.getInfo()
}
