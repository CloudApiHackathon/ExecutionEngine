package pool

import (
	"context"
	"errors"
	"sync"
)

type workerStatus uint

const (
	statusStopped = iota
	statusStarted
)

type defaultWorkerPool[I interface{}, O interface{}] struct {
	workerCount   int
	status        workerStatus
	taskQueue     chan *Task[I, O]
	taskCount     int
	taskCountLock sync.RWMutex
	workerContext context.Context
	workerCancel  context.CancelFunc
	outputChannel chan O
	eventChannel  chan WorkerEvent
}

func NewDefaultWorkerPool[I interface{}, O interface{}](workerCount int) WorkerPool[I, O] {
	workerContext, workerCancel := context.WithCancel(context.Background())

	w := &defaultWorkerPool[I, O]{
		workerCount:   workerCount,
		status:        statusStopped,
		taskQueue:     make(chan *Task[I, O]),
		taskCount:     0,
		taskCountLock: sync.RWMutex{},
		workerContext: workerContext,
		workerCancel:  workerCancel,
		outputChannel: make(chan O),
		eventChannel:  make(chan WorkerEvent),
	}

	return w
}

func (w *defaultWorkerPool[I, O]) Submit(task *Task[I, O]) error {
	if w.status != statusStarted {
		return errors.New("no new tasks are accepted for stopped or paused worker pool")
	}

	w.taskCountLock.Lock()
	defer w.taskCountLock.Unlock()

	w.taskQueue <- task
	w.taskCount++

	return nil
}

func (w *defaultWorkerPool[I, O]) Start() {
	if w.status != statusStopped {
		return
	}

	for i := range w.workerCount {
		go w.worker(i)
	}
	w.status = statusStarted
}

func (w *defaultWorkerPool[I, O]) Stop() {
	if w.status != statusStarted {
		return
	}

	w.workerCancel()

	w.status = statusStopped
}

func (w *defaultWorkerPool[I, O]) OutputChannel() chan O {
	return w.outputChannel
}

func (w *defaultWorkerPool[I, O]) EventChannel() chan WorkerEvent {
	return w.eventChannel
}

func (w *defaultWorkerPool[I, O]) TaskCount() int {
	w.taskCountLock.RLock()
	defer w.taskCountLock.RUnlock()

	return w.taskCount
}

func (w *defaultWorkerPool[I, O]) worker(id int) {
	for {
		select {
		case <-w.workerContext.Done():
			return
		case task := <-w.taskQueue:
			w.outputChannel <- task.TaskFunction(task.Context, id, task.Input)
			w.taskCountLock.Lock()
			w.taskCount--
			if w.taskCount <= 0 {
				go func() {
					w.eventChannel <- EventAllTaskDone
				}()
			}
			w.taskCountLock.Unlock()
		}
	}
}
