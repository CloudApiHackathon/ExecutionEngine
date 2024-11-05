package container

import (
	"archive/tar"
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type ScriptExecutionResult struct {
	ExitCode int
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
}

func CreateContainer(ctx context.Context, cli *client.Client, image string, environmentVariables []string, containerName string) (error, string) {
	response, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        image,
		Env:          environmentVariables,
		WorkingDir:   containerWorkingDirectory,
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
	}, nil, nil, nil, containerName)
	if err != nil {
		return err, ""
	}

	return nil, response.ID
}

func StartContainer(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return err
	}
	return nil
}

func StopContainer(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return err
	}
	return nil
}

func WriteTextToContainer(ctx context.Context, cli *client.Client, containerID, path, fileName, content string, mode int64) error {
	tarBuffer := bytes.NewBuffer(nil)
	tarWriter := tar.NewWriter(tarBuffer)
	defer tarWriter.Close()

	header := &tar.Header{
		Name: fileName,
		Mode: mode,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		return err
	}

	return cli.CopyToContainer(ctx, containerID, path, tarBuffer, container.CopyToContainerOptions{})
}

func ExecuteScriptInContainerSync(ctx context.Context, cli *client.Client, containerID, scriptPath string) (error, *ScriptExecutionResult) {
	execConfig, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"/bin/bash", scriptPath},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          false,
	})
	if err != nil {
		return err, nil
	}

	hijackedResponse, err := cli.ContainerExecAttach(ctx, execConfig.ID, container.ExecAttachOptions{})
	if err != nil {
		return err, nil
	}
	defer hijackedResponse.Close()

	execInspectResponse, err := cli.ContainerExecInspect(ctx, execConfig.ID)
	if err != nil {
		return err, nil
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if _, err = stdcopy.StdCopy(stdout, stderr, hijackedResponse.Reader); err != nil {
		return err, nil
	}

	return nil, &ScriptExecutionResult{
		ExitCode: execInspectResponse.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}
}

func ExecuteScriptInContainerAsync(ctx context.Context, cli *client.Client, containerID, scriptPath string) (error, string, *types.HijackedResponse) {
	execConfig, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"/bin/bash", scriptPath},
		Detach:       true,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          false,
	})
	if err != nil {
		return err, "", nil
	}

	hijackedResponse, err := cli.ContainerExecAttach(ctx, execConfig.ID, container.ExecAttachOptions{})
	if err != nil {
		return err, "", nil
	}
	return err, execConfig.ID, &hijackedResponse
}
