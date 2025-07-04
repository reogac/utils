package fsm

import (
	"time"
	"unsafe"
)

type EventData struct {
	evType      EventType
	evDat       unsafe.Pointer
	createdTime time.Time
}

func NewEmptyEventData(evType EventType) *EventData {
	return &EventData{
		evType:      evType,
		evDat:       nil,
		createdTime: time.Now(),
	}
}

func NewEventData[T any](evType EventType, value *T) *EventData {
	ev := &EventData{
		evType:      evType,
		createdTime: time.Now(),
	}
	if value != nil {
		ev.evDat = unsafe.Pointer(value)
	}
	return ev
}

func (e *EventData) CreatedTime() time.Time {
	return e.createdTime
}

func (e *EventData) Type() EventType {
	return e.evType
}

func GetEventData[T any](e *EventData) *T {
	if e.evDat == nil {
		return nil
	}
	return (*T)(e.evDat)
}

// clone with new event type (for ExitEvent and EntryEvent)
func (e *EventData) clone(evType EventType) *EventData {
	return &EventData{
		evType:      evType,
		evDat:       e.evDat,
		createdTime: time.Now(),
	}
}
