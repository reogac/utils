package fsm

import (
	"unsafe"
)

type EventData struct {
	evType EventType
	evDat  unsafe.Pointer
}

func NewEmptyEventData(evType EventType) *EventData {
	return &EventData{
		evType: evType,
		evDat:  nil,
	}
}

func NewEventData[T any](evType EventType, value *T) *EventData {
	ev := &EventData{
		evType: evType,
	}
	if value != nil {
		ev.evDat = unsafe.Pointer(value)
	}
	return ev
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

// clone with new event type
func (e *EventData) clone(evType EventType) *EventData {
	return &EventData{
		evType: evType,
		evDat:  e.evDat,
	}
}
