package socketIO

import (
	"reflect"
	"sync"
)

type listener func(...interface{})

type listeners struct {
	listenerType
	listener
	next *listeners
}

type listenerType byte

const (
	listenerTypeOn listenerType = iota
	listenerTypeOnce
)

type eventEmitter struct {
	mutex        *sync.Mutex
	listenersMap map[string]*listeners
}

func newEventEmitter() *eventEmitter {
	return &eventEmitter{
		mutex:        &sync.Mutex{},
		listenersMap: make(map[string]*listeners),
	}
}

func (emitter *eventEmitter) Emit(event string, arg ...interface{}) {
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()

	eventListeners, isFound := emitter.listenersMap[event]
	if isFound && eventListeners != nil {
		ptr := eventListeners
		for ptr != nil {
			ptr.listener(arg...)
			ptr = ptr.next
		}
	}
}

func (emitter *eventEmitter) On(event string, f listener) {
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()

	newListener := &listeners{listenerTypeOn, f, nil}

	eventListeners, isFound := emitter.listenersMap[event]
	if !isFound || eventListeners == nil {
		emitter.listenersMap[event] = newListener

		return
	}

	ptr := eventListeners

	for ptr != nil {
		if reflect.ValueOf(ptr.listener).Pointer() == reflect.ValueOf(f).Pointer() {
			ptr.listenerType = listenerTypeOn
			return
		}
		if ptr.next == nil {
			ptr.next = newListener
			return
		}
		ptr = ptr.next
	}
}

func (emitter *eventEmitter) Once(event string, f listener) {
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()

	newListener := &listeners{listenerTypeOnce, f, nil}

	eventListeners, isFound := emitter.listenersMap[event]
	if !isFound || eventListeners == nil {
		emitter.listenersMap[event] = newListener

		return
	}

	ptr := eventListeners

	for ptr != nil {
		if reflect.ValueOf(ptr.listener).Pointer() == reflect.ValueOf(f).Pointer() {
			ptr.listenerType = listenerTypeOn
			return
		}
		if ptr.next == nil {
			ptr.next = newListener
			return
		}
		ptr = ptr.next
	}
}

func (emitter *eventEmitter) RemoveAllListeners(event string) {
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()

	delete(emitter.listenersMap, event)
}

func (emitter *eventEmitter) RemoveListener(event string, f listener) {
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()

	eventListeners, isFound := emitter.listenersMap[event]
	if isFound {
		ptr := eventListeners
		var prev *listeners = nil

		for ptr != nil {
			if reflect.ValueOf(ptr.listener).Pointer() == reflect.ValueOf(f).Pointer() {
				if prev == nil {
					if ptr.next == nil {
						delete(emitter.listenersMap, event)
						return
					}
					emitter.listenersMap[event] = ptr.next
				} else {
					prev.next = ptr.next
				}
				break
			}
			prev = ptr
			ptr = ptr.next
		}
	}
}
