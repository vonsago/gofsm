package fsm

import (
	log "github.com/sirupsen/logrus"
	"math/rand"
	"strconv"
	"syscall"
	"testing"
	"time"
)

const LOGTemple = "FSM [%s] [%s to %s], task_id=%s"

var taskMockEvent []*Event

func testCallback(ee *Event, e *TransFsmError) {
	if e != nil {
		log.Infof("this is callback %v", e.Msg)
	} else {
		log.Info("this is callback")
	}
}

func trans0(event *Event) (dst string, err error) {
	eventMeta := event.Metadata.(map[string]string)
	ch := make(chan int)
	go func() {
		taskId, _ := strconv.Atoi(eventMeta["task_id"])
		task := taskMockEvent[taskId]
		task.Event = "tryCreateJob"
		task.Src = "waiting"
		task.Dst = "running"
		<-ch
	}()
	ch <- 1
	log.Info("trans0 sleep 1")
	time.Sleep(time.Second * 1)
	log.Infof(LOGTemple, event.Event, event.Src, event.Dst, eventMeta["task_id"])
	return "", nil
}

func trans1(event *Event) (dst string, err error) {
	eventMeta := event.Metadata.(map[string]string)
	ch := make(chan int)
	go func() {
		taskId, _ := strconv.Atoi(eventMeta["task_id"])
		task := taskMockEvent[taskId]
		task.Event = "TaskRun"
		task.Src = "running"
		task.Dst = ""
		<-ch
	}()
	ch <- 1
	log.Info("trans1 sleep 2")
	time.Sleep(time.Second * 2)
	log.Infof(LOGTemple, event.Event, event.Src, event.Dst, eventMeta["task_id"])
	return "", nil
}

func trans2(event *Event) (dst string, err error) {
	eventMeta := event.Metadata.(map[string]string)
	ch := make(chan int)
	go func() {
		taskId, _ := strconv.Atoi(eventMeta["task_id"])
		rand.Seed(time.Now().UnixNano())
		r := rand.Intn(10)
		if r%3 == 0 {
			task := taskMockEvent[taskId]
			task.Event = "TaskClear"
			task.Src = "done"
			task.Dst = ""
			if r%2 == 0 {
				task.Src = "fail"
			}
		}
		<-ch
	}()
	ch <- 1
	log.Info("trans2 sleep 1")
	time.Sleep(time.Second * 1)
	log.Infof(LOGTemple, event.Event, event.Src, event.Dst, eventMeta["task_id"])
	return "", nil
}

func trans3(event *Event) (dst string, err error) {
	eventMeta := event.Metadata.(map[string]string)
	ch := make(chan int)
	go func() {
		taskId, _ := strconv.Atoi(eventMeta["task_id"])

		task := taskMockEvent[taskId]
		rand.Seed(time.Now().UnixNano())
		r := rand.Intn(10)
		if r%3 == 0 {
			task.Event = "tryCreateJob"
			task.Src = "waiting"
			task.Dst = "running"
		}
		<-ch
	}()
	ch <- 1
	log.Info("trans3 sleep 2")
	time.Sleep(time.Second * 2)
	log.Infof(LOGTemple, event.Event, event.Src, event.Dst, eventMeta["task_id"])
	return "", nil
}

func mockWaitCallback(fsm *FSM) error {
	tasks := fsm.GetMetadata("mockTasks").([]*Event)
	fsm.FanIn(tasks...)
	return nil
}

// TestFsm Test Mode local Content
// 1.findNewTask prepare to waiting
// 2.tryCreateJob waiting to running
// 3.TaskRun on running
// 4.TaskClear on fail done
//
// mock data is from ...
func TestFsmLocal(t *testing.T) {
	testMockCount := 10
	events := []EventDesc{
		{
			Name: "findNewTask",
			Src:  []string{"prepare"},
			Dst:  "waiting",
		}, {
			Name: "tryCreateJob",
			Src:  []string{"waiting"},
			Dst:  "running",
		}, {
			Name: "TaskRun",
			Src:  []string{"running"},
			Dst:  "",
		}, {
			Name: "TaskClear",
			Src:  []string{"fail", "done"},
			Dst:  "",
		},
	}
	// register state transition func
	f := NewFSM(events, 1*time.Second, ModeLocal, true, 8)
	// mock tasks
	taskMockEvent = []*Event{}
	for i := 0; i < testMockCount; i++ {
		taskData := make(map[string]string)
		taskData["task_id"] = strconv.Itoa(i)
		taskMockEvent = append(taskMockEvent, &Event{
			FSM: f, ID: taskData["task_id"], Event: "findNewTask",
			Src: "prepare", Dst: "waiting", Metadata: taskData,
			Callback: testCallback,
		})
	}
	f.SetMetadata("mockTasks", taskMockEvent)
	// wait injection mock task
	f.RegisterCallback(WAIT, mockWaitCallback)
	// register status flow strategy
	f.Register("findNewTask", "prepare", "waiting", trans0)
	f.Register("tryCreateJob", "waiting", "running", trans1)
	f.Register("TaskRun", "running", "", trans2)
	f.Register("TaskClear", "fail", "", trans3)
	f.Register("TaskClear", "done", "", trans3)
	// 	f.LoopControl()
	cc := make(chan bool)
	go f.LoopControl()
	go func() {
		time.Sleep(time.Second * 20)
		f.SendSignal(syscall.SIGINT)
		cc <- true
	}()
	<-cc
}

func TestFsmSingle(t *testing.T) {
	events := []EventDesc{
		{
			Name: "tryCreateJob",
			Src:  []string{"waiting"},
			Dst:  "running",
		}, {
			Name: "TaskRun",
			Src:  []string{"running"},
			Dst:  "",
		}, {
			Name: "TaskClear",
			Src:  []string{"fail", "done"},
			Dst:  "",
		},
	}
	// register state transition func
	f := NewFSM(events, 1*time.Second, ModeSingle, true, 8)
	go f.LoopControl()
}

func TestFsmCluster(t *testing.T) {
	events := []EventDesc{
		{
			Name: "tryCreateJob",
			Src:  []string{"waiting"},
			Dst:  "running",
		}, {
			Name: "TaskRun",
			Src:  []string{"running"},
			Dst:  "",
		}, {
			Name: "TaskClear",
			Src:  []string{"fail", "done"},
			Dst:  "",
		},
	}
	// register state transition func
	f := NewFSM(events, 1*time.Second, ModeCluster, true, 8)
	go f.LoopControl()
}
