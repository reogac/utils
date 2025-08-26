package fsm

import (
	"sync"
	"unsafe"
)

type StateType int

type State struct {
	current      StateType    //current state value
	nextEv       *EventData   //next event to be handle immediately
	evLock       sync.Mutex   //for locking an event handling
	mutex        sync.RWMutex //for read/write current state value
	info         unsafe.Pointer
	nextEvSetter func(*State, *EventData)
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
	if s.nextEvSetter == nil {
		panic("SetNextEvent must be called within a FSM callback function")
		return
	}
	s.nextEvSetter(s, event)
}
