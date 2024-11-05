package container

import (
	"ExecutionEngine/log"
	"ExecutionEngine/proto/job"
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	"io"
	"time"
)

func Run(ctx context.Context, cli *client.Client, image string, request *job.JobRequest) (error, *job.JobResponse) {
	log.L().Debug("Creating Docker container", zap.String("request", request.String()))
	err, containerID := CreateContainer(ctx, cli, image, request.EnvironmentVariables, "")
	if err != nil {
		return err, nil
	}

	log.L().Debug("Starting Docker container", zap.String("containerID", containerID))
	if err := StartContainer(ctx, cli, containerID); err != nil {
		return err, nil
	}

	log.L().Debug("Copying source code to container", zap.String("containerID", containerID))
	if err := WriteTextToContainer(ctx, cli, containerID, containerWorkingDirectory, request.SourceCodeFileName, request.SourceCode, 0644); err != nil {
		return err, nil
	}

	// Write scripts to the container
	log.L().Debug("Copying setup script to container", zap.String("containerID", containerID))
	if err := WriteTextToContainer(ctx, cli, containerID, containerWorkingDirectory, setupScriptFileName, request.SetupScript, 0644); err != nil {
		return err, nil
	}
	log.L().Debug("Copying compile script to container", zap.String("containerID", containerID))
	if err := WriteTextToContainer(ctx, cli, containerID, containerWorkingDirectory, compileScriptFileName, request.CompileScript, 0644); err != nil {
		return err, nil
	}
	log.L().Debug("Copying run script to container", zap.String("containerID", containerID))
	if err := WriteTextToContainer(ctx, cli, containerID, containerWorkingDirectory, runScriptFileName, request.RunScript, 0644); err != nil {
		return err, nil
	}

	log.L().Debug("Executing setup script", zap.String("containerID", containerID))
	err, setupScriptResult := ExecuteScriptInContainerSync(ctx, cli, containerID, setupScriptPath)
	if err != nil {
		return err, nil
	}
	log.L().Debug("Executed setup script", zap.String("containerID", containerID))
	if setupScriptResult.ExitCode != 0 {
		err := fmt.Errorf("setup script exited with non-zero code")
		return err, &job.JobResponse{
			Status:             "Internal Error",
			ErrorString:        err.Error(),
			SetupStdout:        setupScriptResult.Stdout.String(),
			SetupStderr:        setupScriptResult.Stderr.String(),
			SetupExitCode:      int32(setupScriptResult.ExitCode),
			CompileStdout:      "",
			CompileStderr:      "",
			CompileExitCode:    -1,
			RunStdout:          "",
			RunStderr:          "",
			RunExitCode:        -1,
			ResourceStatistics: nil,
		}
	}

	err, compileScriptResult := ExecuteScriptInContainerSync(ctx, cli, containerID, compileScriptPath)
	if err != nil {
		return err, nil
	}
	log.L().Debug("Executed compile script", zap.String("compileScriptResult", fmt.Sprintf("%#v", compileScriptResult)))
	if compileScriptResult.ExitCode != 0 {
		err := fmt.Errorf("compile script exited with non-zero code")
		return err, &job.JobResponse{
			Status:             "Compile Error",
			ErrorString:        err.Error(),
			SetupStdout:        setupScriptResult.Stdout.String(),
			SetupStderr:        setupScriptResult.Stderr.String(),
			SetupExitCode:      int32(setupScriptResult.ExitCode),
			CompileStdout:      compileScriptResult.Stdout.String(),
			CompileStderr:      compileScriptResult.Stderr.String(),
			CompileExitCode:    int32(compileScriptResult.ExitCode),
			RunStdout:          "",
			RunStderr:          "",
			RunExitCode:        -1,
			ResourceStatistics: nil,
		}
	}

	log.L().Debug("Executed run script", zap.String("compileScriptResult", fmt.Sprintf("%#v", compileScriptResult)))
	cancelContext, cancel := context.WithTimeout(ctx, time.Duration(request.ResourceLimits.MaxExecutionTime)*time.Millisecond)
	startTime := time.Now()
	err, _, hijackedResponse := ExecuteScriptInContainerAsync(cancelContext, cli, containerID, runScriptPath)
	if err != nil {
		cancel()
		return err, nil
	}
	defer hijackedResponse.Close()

	containerWaitChannel, containerErrorChannel := cli.ContainerWait(cancelContext, containerID, container.WaitConditionNotRunning)

	// Monitor and collect data from the execution of run script
	goroutineErrorChannel := make(chan error)

	go func() {
		_, err = io.Copy(hijackedResponse.Conn, bytes.NewReader([]byte(request.Stdin)))
		if err != nil {
			goroutineErrorChannel <- err
			cancel()
		}
		err := hijackedResponse.CloseWrite()
		if err != nil {
			goroutineErrorChannel <- err
			cancel()
		}
	}()

	runScriptStdoutBuffer := &bytes.Buffer{}
	runScriptStderrBuffer := &bytes.Buffer{}
	go func() {
		_, err := stdcopy.StdCopy(runScriptStdoutBuffer, runScriptStderrBuffer, hijackedResponse.Conn)
		if err != nil {
			goroutineErrorChannel <- err
			cancel()
		}
	}()

	for {
		select {
		case waitResponse := <-containerWaitChannel:
			errorString := ""
			if waitResponse.Error != nil {
				errorString = waitResponse.Error.Message
			}
			return nil, &job.JobResponse{
				Status:          "Finished",
				ErrorString:     errorString,
				SetupStdout:     setupScriptResult.Stdout.String(),
				SetupStderr:     setupScriptResult.Stderr.String(),
				SetupExitCode:   int32(setupScriptResult.ExitCode),
				CompileStdout:   compileScriptResult.Stdout.String(),
				CompileStderr:   compileScriptResult.Stderr.String(),
				CompileExitCode: int32(compileScriptResult.ExitCode),
				RunStdout:       runScriptStdoutBuffer.String(),
				RunStderr:       runScriptStderrBuffer.String(),
				RunExitCode:     int32(waitResponse.StatusCode),
				ResourceStatistics: &job.ResourceStatistics{
					ExecutionTime: time.Since(startTime).Milliseconds(),
					MaxMemoryUsed: -1,
				},
			}
		case err := <-containerErrorChannel:
			cancel()
			return err, &job.JobResponse{
				Status:          "Aborted",
				ErrorString:     err.Error(),
				SetupStdout:     setupScriptResult.Stdout.String(),
				SetupStderr:     setupScriptResult.Stderr.String(),
				SetupExitCode:   int32(setupScriptResult.ExitCode),
				CompileStdout:   compileScriptResult.Stdout.String(),
				CompileStderr:   compileScriptResult.Stderr.String(),
				CompileExitCode: int32(compileScriptResult.ExitCode),
				RunStdout:       runScriptStdoutBuffer.String(),
				RunStderr:       runScriptStderrBuffer.String(),
				RunExitCode:     -1,
				ResourceStatistics: &job.ResourceStatistics{
					ExecutionTime: time.Since(startTime).Milliseconds(),
					MaxMemoryUsed: -1,
				},
			}
		case err := <-goroutineErrorChannel:
			cancel()
			return err, &job.JobResponse{
				Status:          "Aborted",
				ErrorString:     err.Error(),
				SetupStdout:     setupScriptResult.Stdout.String(),
				SetupStderr:     setupScriptResult.Stderr.String(),
				SetupExitCode:   int32(setupScriptResult.ExitCode),
				CompileStdout:   compileScriptResult.Stdout.String(),
				CompileStderr:   compileScriptResult.Stderr.String(),
				CompileExitCode: int32(compileScriptResult.ExitCode),
				RunStdout:       runScriptStdoutBuffer.String(),
				RunStderr:       runScriptStderrBuffer.String(),
				RunExitCode:     -1,
				ResourceStatistics: &job.ResourceStatistics{
					ExecutionTime: time.Since(startTime).Milliseconds(),
					MaxMemoryUsed: -1,
				},
			}
		}
	}
}
