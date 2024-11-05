package server

import (
	"ExecutionEngine/container"
	"ExecutionEngine/log"
	"ExecutionEngine/proto/job"
	"context"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type taskInput struct {
	ID      uuid.UUID
	Request *job.JobRequest
}

type taskOutput struct {
	ID       uuid.UUID
	Error    error
	Response *job.JobResponse
}

func task(ctx context.Context, input *taskInput) *taskOutput {
	log.L().Debug("Creating Docker client", zap.String("taskID", input.ID.String()))
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return &taskOutput{
			input.ID,
			err,
			nil,
		}
	}

	log.L().Debug("Running Docker container", zap.String("taskID", input.ID.String()))
	err, response := container.Run(ctx, cli, dockerImageName, input.Request)
	if err != nil {
		return &taskOutput{
			input.ID,
			err,
			nil,
		}
	}

	log.L().Debug("Got task output", zap.String("taskID", input.ID.String()), zap.String("response", response.String()))
	return &taskOutput{
		input.ID,
		nil,
		response,
	}
}
