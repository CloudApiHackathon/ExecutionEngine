package server

import (
	"ExecutionEngine/container"
	"ExecutionEngine/log"
	"ExecutionEngine/pool"
	"ExecutionEngine/proto/job"
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"runtime"
)

type Server struct {
	job.UnimplementedJobServer

	listener   net.Listener
	grpcServer *grpc.Server
	pool       pool.WorkerPool[*taskInput, *taskOutput]
}

func NewServer() *Server {
	return &Server{
		pool: pool.NewDefaultWorkerPool[*taskInput, *taskOutput](runtime.NumCPU()),
	}
}

func (s *Server) Initialize() {
	log.L().Debug("Initializing server", zap.String("listenAddress", listenAddress))

	log.L().Debug("Creating Docker client")
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("failed to create Docker client: %w", err))
	}

	log.L().Debug("Building Docker image")
	err = container.BuildImage(context.Background(), cli, dockerBuildContextFolder, dockerImageName)
	if err != nil {
		panic(fmt.Errorf("failed to build Docker image: %w", err))
	}

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.L().Panic("Cannot listen on address", zap.Error(err), zap.String("listenAddress", listenAddress))
	}
	s.listener = listener

	s.grpcServer = grpc.NewServer()
	job.RegisterJobServer(s.grpcServer, s)
}

func (s *Server) Serve() {
	log.L().Info("Starting server", zap.String("listenAddress", listenAddress))

	log.L().Info("Starting worker pool")
	s.pool.Start()

	go func() {
		err := s.grpcServer.Serve(s.listener)
		if err != nil {
			log.L().Panic("Cannot start gRPC server", zap.Error(err))
		}
	}()

outer:
	for {
		select {
		case item := <-s.pool.OutputChannel():
			fmt.Print(item.ID)
		case event := <-s.pool.EventChannel():
			switch event {
			case pool.EventAllTaskDone:
				break outer
			}
		}
	}
}

func (s *Server) Submit(ctx context.Context, request *job.JobRequest) (*job.JobResponse, error) {
	log.L().Debug("Received new gRPC call", zap.String("request", request.String()))
	taskID := uuid.New()
	// TODO: Use worker pool
	output := task(ctx, &taskInput{
		taskID,
		request,
	})
	if output.Error != nil {
		log.L().Error("Task failed", zap.Error(output.Error), zap.String("taskID", taskID.String()))
	}

	return output.Response, output.Error
}
