package fsm

import (
	log "github.com/sirupsen/logrus"

	"container/list"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type TransCostType struct {
	id    string
	trans string
	cost  time.Duration
}

const (
	ModeLocal   = "local"
	ModeSingle  = "singleton"
	ModeCluster = "cluster"

	NEW   = "new"
	READY = "ready"
	RUN   = "running"
	WAIT  = "waiting"
	QUIT  = "quit"

	FSMetaErrors = "errors"
	FSMetaCost   = "fsm:cost"
)

type FSM struct {
	// mode
	Mode string
	// current is the state that the FSM is currently in.
	// enum (new, run, wait, quit)
	current string
	// the interval every scheduler round
	interval time.Duration
	// showFlow 1 turn on show status flows.
	showFlow bool
	// fanCount
	fanCount int

	// cluster
	cluster ClusterConf

	// allEvents and allStates record FSM strategy
	allEvents map[string]bool
	allStates map[string]bool
	// callback func will be executed in specific FSM current state
	// register callback use RegisterCallback()
	callback map[string]func(fsm *FSM) error
	// transitions maps events and source states to destination states.
	transitions map[eKey]string
	// transition is the internal transition functions used either directly
	// or when Transition is called in an asynchronous state transition.
	transition map[string]func(event *Event) (dst string, err error)
	// transCost is TransCostType which recorded events' cost from source
	// states to destination states. options need RecordCost
	transCost *list.List

	// eventFan
	// stateMu guards access to the current state.
	// eventMu guards access to Event() and Transition().
	eventFan   chan *Event
	fanWg      sync.WaitGroup
	eventCount int
	stateMu    sync.RWMutex
	eventMu    sync.RWMutex
	// metadata can be used to store and load data that maybe used across events
	// use methods SetMetadata() and Metadata() to store and load data
	// FSMetaErrors are needed
	metadata   map[string]interface{}
	metadataMu sync.RWMutex

	// signal chan for listen terminal options
	sigC chan os.Signal
}

func stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func (f *FSM) recordPanic() {
	if err := recover(); err != nil {
		log.Errorf("%s, %v", string(stack()), err)
		panic(err)
	}
}

func (f *FSM) showReport() {
	transCount := 0
	transMetric := make(map[string]map[string]time.Duration)
	for e := f.transCost.Front(); e != nil; e = e.Next() {
		ts := e.Value.(TransCostType)
		if val, ok := transMetric[ts.trans]; ok {
			if val["max"] < ts.cost {
				transMetric[ts.trans]["max"] = ts.cost
			}
			if val["min"] > ts.cost {
				transMetric[ts.trans]["min"] = ts.cost
			}
			transMetric[ts.trans]["avg"] = (val["avg"]*time.Duration(transCount) + ts.cost) /
				time.Duration(transCount+1)
		} else {
			transMetric[ts.trans] = make(map[string]time.Duration)
			transMetric[ts.trans]["avg"], transMetric[ts.trans]["min"], transMetric[ts.trans]["max"] =
				ts.cost, ts.cost, ts.cost
		}
		transCount += 1
	}
	reportTemple := fmt.Sprintf("\n  FSM Report:  %d Finished\n", transCount)
	for k, v := range transMetric {
		reportTemple += fmt.Sprintf(
			"  | Report event %s, cost max: %v, avg: %v, min %v |\n",
			k, v["max"], v["avg"], v["min"],
		)
	}
	log.Info(reportTemple)

	// clear cost records
	f.transCost.Init()
}

func (f *FSM) signalHandler() {
	signal.Notify(f.sigC, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1)
	for sig := range f.sigC {
		switch sig {
		case syscall.SIGUSR1:
			time.Sleep(2 * time.Second)
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			f.quit()
		default:
			log.Infof("signalHandler receive signal %v", sig)
		}
	}
}

func (f *FSM) SendSignal(sig os.Signal) {
	f.sigC <- sig
}

func (f *FSM) Current() string {
	f.stateMu.Lock()
	c := f.current
	f.stateMu.Unlock()
	return c
}

func (f *FSM) setCurrent(s string) {
	f.stateMu.Lock()
	f.current = s
	f.stateMu.Unlock()
}

func (f *FSM) CmpAndSwp(s string, ds string) string {
	f.stateMu.Lock()
	if f.current == s {
		f.current = ds
	}
	c := f.current
	f.stateMu.Unlock()
	return c
}

func (f *FSM) RecordCost(id string, k eKey, dst string, t time.Duration) {
	f.metadataMu.Lock()
	m := TransCostType{id, f.GetTransName(k, dst), t}
	f.transCost.PushBack(m)
	f.metadataMu.Unlock()
}

func (f *FSM) SetMetadata(key string, value interface{}) {
	f.metadataMu.Lock()
	f.metadata[key] = value
	f.metadataMu.Unlock()
}

func (f *FSM) GetMetadata(key string) interface{} {
	f.metadataMu.Lock()
	val, ok := f.metadata[key]
	f.metadataMu.Unlock()
	if !ok {
		val = nil
	}
	return val
}

func (f *FSM) GetCost() (c int64) {
	f.metadataMu.Lock()
	defer f.metadataMu.Unlock()
	if val, ok := f.metadata[FSMetaCost]; ok {
		return val.(int64)
	}
	return c
}

// Register
// It injects the event executor which is responsible for transit state from src to dst.
func (f *FSM) Register(event string, src string, dst string, exec func(event *Event) (dst string, err error)) {
	e := eKey{event, src}
	if v, ok := f.transitions[e]; ok && v == dst {
		f.transition[f.GetTransName(e, dst)] = exec
	} else {
		panic(ErrFSMRegisterError)
	}
}

// RegisterCallback
// callback when FSM on state ( NEW, READY, WAIT, RUN, QUIT)
func (f *FSM) RegisterCallback(c string, callback func(fsm *FSM) error) {
	switch c {
	case NEW, READY, WAIT, RUN, QUIT:
		f.callback[c] = callback
	default:
		panic(ErrFSMRegisterError)
	}
}

// FanIn
// event to chan
func (f *FSM) FanIn(events ...*Event) {
	l := len(events)
	f.eventCount += l
	f.fanWg.Add(l)
	go func() {
		defer f.recordPanic()
		for _, n := range events {
			f.eventFan <- n
		}
	}()
}

// FanOut
// event to channel
func (f *FSM) FanOut() {
	go func() {
		defer f.recordPanic()
		f.eventMu.RLock()
		defer f.eventMu.RUnlock()
		for {
			e, open := <-f.eventFan
			if !open {
				break
			}
			go func(event *Event) {
				defer f.fanWg.Done()
				defer f.recordPanic()
				st := time.Now()
				ek := eKey{event.Event, event.Src}
				if ed, ok := f.transitions[ek]; ok {
					dst, err := f.transition[f.GetTransName(ek, ed)](event)
					if err != nil || dst != event.Dst {
						TransError := &TransFsmError{
							Msg:    "transition error occur",
							Detail: err,
							Dst:    dst,
						}
						event.Callback(event, TransError)
					} else {
						event.Callback(event, nil)
					}
					tc := time.Since(st)
					// todo use channel instead of lock
					f.RecordCost(event.ID, ek, dst, tc)
				} else {
					log.Warnf(
						"FSM Run but not found transition %s %s",
						event.Event, event.Src,
					)
				}
			}(e)
		}
	}()
}

// GetTransName
// state flow execution function name
// The name's struct is "T+eKey.event+From+Src+To+dst",
// if dst is "" it changes to "T+eKey.event+On+Src"
func (f *FSM) GetTransName(e eKey, dst string) string {
	if dst == "" {
		return fmt.Sprintf("T%sOn%s", e.event, e.src)
	} else {
		return fmt.Sprintf("T%sFrom%sTo%s", e.event, e.src, dst)
	}
}

// selfCheck
// This method is the pre-process before FSM round which is contains
// user's callback and register signal listen.
// In addition, it checks if the registered, status, events or transitions is legal.
func (f *FSM) selfCheck() {
	// flow legal check
	for k, v := range f.transitions {
		if _, ok := f.transition[f.GetTransName(k, v)]; !ok {
			panic(fmt.Sprintf("%v, event %s from [%s] to [%s]",
				ErrFSMTransitionNotRegister, k.event, k.src, v))
		}
	}
	// If mode is singleton, the NEW callback is needed.
	// User can implement own logic to make sure only one fsm running at a moment.
	if f.Mode == ModeSingle {
		if callback, ok := f.callback[READY]; ok {
			panic(ErrFSMCallbackNewNeeded)
		} else {
			if e := callback(f); e != nil {
				log.Error("SelfCheck FSM NEW callback error")
			}
		}
	}
	// callback ready
	if callback, ok := f.callback[READY]; ok {
		if e := callback(f); e != nil {
			panic(fmt.Sprintf("%v, cause SelfCheck FSM READY callback error", ErrStateNotAvailable))
		}
	}
	_ = f.CmpAndSwp(NEW, READY)
}

func (f *FSM) wait() error {
	// init channel and count
	f.eventMu.Lock()
	f.eventFan = make(chan *Event)
	f.eventMu.Unlock()
	f.eventCount = 0
	c := f.CmpAndSwp(READY, WAIT)
	if c != WAIT {
		return ErrFSMIsRunning
	}
	// wait callback
	if callback, ok := f.callback[WAIT]; ok {
		if e := callback(f); e != nil {
			return e
		}
	}
	// show flows
	if f.showFlow {
		f.showReport()
	}
	return nil
}

func (f *FSM) RunFan() error {
	defer f.setCurrent(READY)
	f.CmpAndSwp(WAIT, RUN)
	for i := 0; i < f.fanCount; i++ {
		f.FanOut()
	}
	f.fanWg.Wait()
	close(f.eventFan)
	log.Infof("FSM Run event count: %d", f.eventCount)
	// run callback
	if callback, ok := f.callback[RUN]; ok {
		if e := callback(f); e != nil {
			return e
		}
	}
	return nil
}

func (f *FSM) quit() {
	if f.Current() == QUIT {
		return
	}
	// wait run finished
	for {
		s := f.CmpAndSwp(READY, QUIT)
		if s == QUIT {
			break
		} else {
			time.Sleep(time.Second * f.interval)
		}
	}
	// quit callback
	if callback, ok := f.callback[QUIT]; ok {
		if err := callback(f); err != nil {
			log.Error("Quit FSM quit callback error")
		}
	}
	log.Info("Quit FSM quit")
}

// LoopControl
//
//	 ├──> selfCheck() [callback[READY]] ──> return err
//	 ├
//	 ├─Loop─> wait() [callback[WAIT]] ──> Run() [callback[RUN]]
//		         └──> showReport()
//	 ├  *setCurrent(QUIT) to quit loop
//	 └──> quit() [callback[QUIT]]
func (f *FSM) LoopControl() {
	go f.signalHandler()
	f.selfCheck()
	for {
		st := time.Now().UnixMilli()
		c := f.Current()
		if c == QUIT {
			break
		} else if c == WAIT {
			continue
		}
		f.setCurrent(READY)
		// TODO refactor async
		_ = f.wait()
		_ = f.RunFan()
		cost := time.Now().UnixMilli() - st
		f.SetMetadata(FSMetaCost, cost)
		log.Infof("FSM Run cost: %d ms", cost)
		time.Sleep(f.interval)
	}
	f.quit()
}

func NewFSM(events []EventDesc, interval time.Duration, mode string, showFlow bool, fanCount int) *FSM {
	f := &FSM{
		Mode:        mode,
		current:     NEW,
		interval:    interval,
		showFlow:    showFlow,
		fanCount:    fanCount,
		transCost:   list.New(),
		transitions: make(map[eKey]string),
		callback:    make(map[string]func(fmt *FSM) error),
		transition:  make(map[string]func(event *Event) (dst string, err error)),
		metadata:    make(map[string]interface{}),
		eventFan:    make(chan *Event),
		eventCount:  0,
		allEvents:   make(map[string]bool),
		allStates:   make(map[string]bool),
		sigC:        make(chan os.Signal, 1),
	}
	// Cache tasks list for run consume
	// Notice memory size
	f.SetMetadata(FSMetaErrors, list.New())
	// Build transition map and store sets of all events and states.
	for _, e := range events {
		for _, src := range e.Src {
			f.transitions[eKey{e.Name, src}] = e.Dst
			f.allStates[src] = true
			f.allStates[e.Dst] = true
		}
		f.allEvents[e.Name] = true
	}
	return f
}

// Event is the info that get passed as a reference in the callbacks.
type Event struct {
	// FSM is a reference to the current FSM.
	FSM *FSM

	//ID event identifier
	ID string

	// Event is the event name.
	Event string

	// the event's priority
	Priority int

	// Src is the state before the transition.
	Src string

	// Dst is the state after the transition.
	Dst string

	// Metadata is the common data as event init
	Metadata interface{}
	// MetaMu guards access to the event metadata.
	MetaMu sync.RWMutex

	// Callback is the Event's callback
	Callback func(event *Event, e *TransFsmError)

	// Canceled is an internal flag set if the transition is canceled.
	Canceled bool

	// Msg for describe the trans
	Msg string
}

// EventDesc represents an event when initializing the FSM.
//
// The event can have one or more source states that is valid for performing
// the transition. If the FSM is in one of the source states it will end up in
// the specified destination state, calling all defined callbacks as it goes.
type EventDesc struct {
	// Name is the event name used when calling for a transition.
	Name string

	// Src is a slice of source states that the FSM must be in to perform a
	// state transition.
	Src []string

	// Dst is the destination state that the FSM will be in if the transition
	// succeeds.
	Dst string
}

// eKey is a struct key used for storing the transition map.
type eKey struct {
	// event is the name of the event that the keys refers to.
	event string

	// src is the source from where the event can transition.
	src string
}
