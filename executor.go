package goxecutor

import (
	"sync"
)

type Task func()
type PriorityTask struct {
	Task     Task
	Priority int
}

const signal = 0
const signalNewTask = 1
const signalWorkerFree = 2

type Executor interface {
	Submit(task Task) Executor
	SubmitWithPriority(task Task, priority int) Executor

	Wait()
	Destroy()
}

type executor struct {
	destroyed    bool
	destroyMutex *sync.Mutex

	maxPoolSize uint32
	poolSize    uint32
	freeWorkers uint32

	poolMutex *sync.RWMutex

	tasks      []PriorityTask
	tasksMutex *sync.Mutex

	waitTasks *sync.WaitGroup

	handlerReceiver chan int
	newTaskReceiver chan int // new task channel signal

}

func (e *executor) Wait() {
	e.waitTasks.Wait()
}

func NewFixedPoolExecutor(workers uint32) Executor {
	if workers == 0 {
		panic("max goroutines cannot be 0")
	}

	e := newExecutor(workers)

	return e
}

func NewSingleThreadExecutor() Executor {
	e := newExecutor(1)
	return e
}

func newExecutor(maxPoolSize uint32) *executor {
	e := &executor{
		tasks:       []PriorityTask{},
		maxPoolSize: maxPoolSize,

		poolMutex:  &sync.RWMutex{},
		tasksMutex: &sync.Mutex{},

		waitTasks: &sync.WaitGroup{},

		destroyMutex: &sync.Mutex{},

		handlerReceiver: make(chan int, 0),
		newTaskReceiver: make(chan int, 0),
	}

	go e.startTaskHandler()

	return e
}

func (e *executor) startTaskHandler() {
	for {
		select {
		case msg := <-e.handlerReceiver:
			if msg == signalNewTask {
				e.onNewTaskHandler()
			} else if msg == signalWorkerFree {
				e.onWorkerFreeHandler()
			}

			break
		}
	}
}

func (e *executor) onWorkerFreeHandler() {
	e.poolMutex.Lock()
	e.freeWorkers++
	e.poolMutex.Unlock()

	if e.destroyed && e.poolSize == e.freeWorkers {
		return
	}
}

func (e *executor) onNewTaskHandler() {
	e.poolMutex.Lock()
	if e.freeWorkers > 0 {
		e.freeWorkers--
		e.newTaskReceiver <- signal
	} else if e.poolSize < e.maxPoolSize {
		e.poolSize += 1

		go e.startWorker()
		e.newTaskReceiver <- signal
	}

	e.poolMutex.Unlock()
}

// Submit adds a task to the executor with priority 0
func (e *executor) Submit(task Task) Executor {
	e.SubmitWithPriority(task, 0)
	return e
}

func (e *executor) SubmitWithPriority(task Task, priority int) Executor {
	if e.destroyed {
		panic("cannot submit tasks on a destroyed executor")
	}

	e.waitTasks.Add(1)
	e.enqueueTask(PriorityTask{
		Task:     task,
		Priority: priority,
	})

	e.handlerReceiver <- signalNewTask
	return e
}

func (e *executor) Destroy() {
	e.destroyMutex.Lock()
	defer e.destroyMutex.Unlock()
	e.tasksMutex.Lock()
	defer e.tasksMutex.Unlock()

	e.destroyed = true
	num := len(e.tasks)

	e.tasks = make([]PriorityTask, 0)
	e.waitTasks.Add(-num)
}

func (e *executor) startWorker() {
	for {
		select {
		case <-e.newTaskReceiver:
			for e.processNextTask() {
			}

			e.destroyMutex.Lock()
			if e.destroyed {
				e.destroyMutex.Unlock()
				return
			} else {
				e.handlerReceiver <- signalWorkerFree
			}
			e.destroyMutex.Unlock()

			break
		}
	}
}

func (e *executor) processNextTask() bool {
	task := e.popNextTask()
	if task == nil {
		return false
	}

	defer func() {
		e.waitTasks.Done()
	}()

	if e.destroyed {
		return true
	}

	(*task)()
	return true
}

func (e *executor) enqueueTask(task PriorityTask) {
	e.tasksMutex.Lock()
	defer e.tasksMutex.Unlock()

	if len(e.tasks) == 0 {
		e.tasks = append(e.tasks, task)
		return
	}

	// check priority
	var i = 0
	for _, t := range e.tasks {
		if t.Priority < task.Priority {
			break
		}
		i++
	}

	if i == len(e.tasks) {
		e.tasks = append(e.tasks, task)
		return
	}

	e.tasks = append(e.tasks[:i+1], e.tasks[i:]...)
	e.tasks[i] = task
}

func (e *executor) popNextTask() *Task {
	e.tasksMutex.Lock()
	defer e.tasksMutex.Unlock()

	if len(e.tasks) == 0 {
		return nil
	}

	t := e.tasks[0]
	e.tasks = e.tasks[1:]

	return &t.Task
}
