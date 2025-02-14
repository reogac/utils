package fsm

import (
	"fmt"
	"sync"
	"unsafe"
)

type StateType int
type EventType int

const (
	EntryEvent EventType = iota
	ExitEvent
	EventIndexStart
)

type State struct {
	current StateType    //current state value
	next    *EventData   //next event to be handle immediately
	evLock  sync.Mutex   //for locking an event handling
	mutex   sync.RWMutex //for read/write current state value
	info    unsafe.Pointer
}

func NewState[T any](i StateType, info *T) *State {
	state := &State{
		current: i,
	}
	if info != nil {
		state.info = unsafe.Pointer(info)
	}
	return state
}

func GetStateInfo[T any](state *State) *T {
	if state.info == nil {
		return nil
	}
	return (*T)(state.info)
}

func (s *State) setState(now StateType) {
	s.mutex.Lock()
	s.current = now
	s.mutex.Unlock()
}

func (s *State) CurrentState() StateType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.current
}

func (s *State) SetNextEvent(event *EventData) {
	s.next = event
}

type StateEventTuple struct {
	state StateType
	event EventType
}

func Tuple(state StateType, event EventType) (tuple StateEventTuple) {
	tuple.event = event
	tuple.state = state
	return
}

type Transitions map[StateEventTuple]StateType
type CallbackFn func(*State, *EventData)
type Callbacks map[StateType]CallbackFn

type Fsm struct {
	transitions Transitions
	callbacks   Callbacks
	events      map[EventType]bool
	handler     CallbackFn
	done        chan struct{}
	w           Executer
}

type Options struct {
	Transitions           Transitions
	Callbacks             Callbacks
	GenericCallback       CallbackFn
	NonTransitionalEvents []EventType
}

func NewFsm(opts Options, w Executer) *Fsm {
	ret := &Fsm{
		transitions: make(map[StateEventTuple]StateType),
		callbacks:   make(map[StateType]CallbackFn),
		events:      make(map[EventType]bool),
		handler:     opts.GenericCallback,
		done:        make(chan struct{}),
		w:           w,
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

	// set a generic handler and a list of non-transitional events that will be
	// handled by the handler
	for _, ev := range opts.NonTransitionalEvents {
		if _, ok := knownEvents[ev]; ok {
			panic("Non transional event must not in the transision list")
		} else {
			ret.events[ev] = true
		}
	}

	//go ret.loop()
	return ret
}

type Executer interface {
	Submit(func())
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
	for state.next != nil {
		next := state.next
		state.next = nil //reset next event for the state
		if _, ok := fsm.events[next.Type()]; ok {
			fsm.handler(state, next)
		} else { //if it is a transitional event
			fsm.transit(state, next, nil)
		}
	}
}

func (fsm *Fsm) handleEvent(state *State, event *EventData, errCh chan error, sync bool) {
	//a state only process one event at a time, so we need to lock it
	//release the state lock after finish handling the event

	var fn func()
	var nonTransit bool
	//if the event is in the list of non-transitional events
	if _, nonTransit = fsm.events[event.Type()]; nonTransit {
		fn = func() {
			state.evLock.Lock()
			fsm.handler(state, event)
			fsm.processNextEvent(state)
			state.evLock.Unlock()
		}
	} else { //if it is a transitional event
		fn = func() {
			state.evLock.Lock()
			fsm.transit(state, event, errCh)
			fsm.processNextEvent(state)
			state.evLock.Unlock() //unlock the state
		}
	}
	if sync {
		fn()
	} else {
		fsm.w.Submit(fn) //handle the event in a worker pool
	}
	if nonTransit {
		errCh <- nil
	}
}

func (fsm *Fsm) transit(state *State, event *EventData, errCh chan error) {
	current := state.CurrentState()
	tuple := StateEventTuple{
		state: current,
		event: event.Type(),
	}

	if nextState, ok := fsm.transitions[tuple]; ok {
		if errCh != nil {
			errCh <- nil
		}
		//execute callback for the event
		curCallback := fsm.callbacks[current]
		nextCallback := fsm.callbacks[nextState]
		if curCallback != nil {
			curCallback(state, event)
		}
		if current != nextState { //state will be changed
			//exectute callback for ExitEvent of the current state
			//log.Tracef("EXIT state %d", current)
			curCallback(state, event.clone(ExitEvent))
			//change to the next state
			//log.Tracef("ENTRER state %d", nextState)
			state.setState(nextState)
			//execute callback for EtryEvent of the next state
			if nextCallback != nil {
				//log.Tracef("Call ENTRY event on state %d", nextState)
				nextCallback(state, event.clone(EntryEvent))
			}
		}
	} else {
		if errCh != nil {
			errCh <- fmt.Errorf("Unknown transition from state %v with event %v", current, event)
		}
	}
}
