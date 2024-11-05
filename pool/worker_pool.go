package pool

import (
	"context"
)

type WorkerEvent uint

const (
	EventAllTaskDone = iota
)

type TaskFunction[I interface{}, O interface{}] func(context context.Context, workerID int, input I) O
type Task[I interface{}, O interface{}] struct {
	Context      context.Context
	TaskFunction TaskFunction[I, O]
	Input        I
}

type WorkerPool[I interface{}, O interface{}] interface {
	Submit(task *Task[I, O]) error
	Start()
	Stop()
	OutputChannel() chan O
	EventChannel() chan WorkerEvent
	TaskCount() int
}
