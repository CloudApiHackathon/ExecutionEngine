package container

import (
	"ExecutionEngine/log"
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func BuildImage(ctx context.Context, cli *client.Client, dockerBuildContextFolder, imageName string) error {
	log.L().Debug("Creating build context (tar archive)", zap.String("dockerBuildContextFolder", dockerBuildContextFolder))
	err, tarBuffer := createBuildContext(dockerBuildContextFolder)
	if err != nil {
		return fmt.Errorf("failed to create build context: %w", err)
	}

	log.L().Debug("Building Docker image")
	imageBuildResponse, err := cli.ImageBuild(ctx, tarBuffer, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageName},
		Version:    types.BuilderV1,
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer imageBuildResponse.Body.Close()

	// Reading build output
	scanner := bufio.NewScanner(imageBuildResponse.Body)
	for scanner.Scan() {
		text := scanner.Text()

		var message jsonmessage.JSONMessage
		err := json.Unmarshal([]byte(text), &message)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSONMessage: %w", err)
		}
		if message.Error != nil {
			return message.Error
		}
		if message.Stream != "" {
			log.L().Info(strings.ReplaceAll(message.Stream, "\n", ""))
		}
	}

	return nil
}

// createBuildContext creates a tar archive of the build context.
func createBuildContext(dockerBuildContextFolder string) (error, io.Reader) {
	buffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buffer)
	defer tarWriter.Close()

	err := filepath.Walk(dockerBuildContextFolder, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			log.L().Debug("Processing directory", zap.String("path", path))
			return nil
		}

		log.L().Debug("Adding file to build context", zap.String("path", path))

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		header, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
		if err != nil {
			return err
		}
		header.Name, _ = filepath.Rel(dockerBuildContextFolder, path)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if _, err := io.Copy(tarWriter, f); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk context directory: %w", err), nil
	}

	return nil, buffer
}
