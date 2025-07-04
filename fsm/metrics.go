package fsm

import (
	"math"
	"sync"
	"time"
)

type FsmMetrics struct {
	submitted int64
	triggered int64
	completed int64
	evMetrics map[EventType]*EventMetrics
	mutex     sync.Mutex
}

func newFsmMetrics() FsmMetrics {
	return FsmMetrics{
		evMetrics: make(map[EventType]*EventMetrics),
	}
}

func (m *FsmMetrics) onTriggered() {
	m.mutex.Lock()
	m.triggered++
	m.mutex.Unlock()
}

func (m *FsmMetrics) onSubmitted() {
	m.mutex.Lock()
	m.submitted++
	m.mutex.Unlock()
}

func (m *FsmMetrics) onCompleted(evType EventType, started time.Time) {
	m.mutex.Lock()
	m.completed++
	ev, ok := m.evMetrics[evType]
	if !ok {
		ev = new(EventMetrics)
		m.evMetrics[evType] = ev
	}
	ev.add(time.Now().Sub(started).Nanoseconds())
	m.mutex.Unlock()
}

type FsmInfo struct {
	NumSubmitted int64
	NumTriggered int64
	NumCompleted int64
	EvStats      []EventInfo
}

func (m *FsmMetrics) getInfo() *FsmInfo {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	info := &FsmInfo{
		NumSubmitted: m.submitted,
		NumTriggered: m.triggered,
		NumCompleted: m.completed,
	}
	info.EvStats = make([]EventInfo, len(m.evMetrics))
	var i int = 0
	for evType, stats := range m.evMetrics {
		info.EvStats[i] = EventInfo{
			EvType:     int(evType),
			Count:      stats.cnt,
			ResetCount: stats.resetCnt,
			Duration:   stats.sumDuration,
		}
	}
	return info
}

type EventMetrics struct {
	cnt         uint32
	sumDuration int64
	resetCnt    uint16
}

func (m *EventMetrics) add(duration int64) {
	if m.cnt == math.MaxUint32 {
		//reset
		m.cnt = 0
		m.sumDuration = 0
		m.resetCnt++
	}

	m.cnt++
	m.sumDuration += duration
}

type EventInfo struct {
	EvType     int
	Count      uint32
	Duration   int64
	ResetCount uint16
}
