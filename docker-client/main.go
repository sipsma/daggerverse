// A module wrapping docker client operations; mainly meant for demoing unix socket
// arg usage right now.

package main

import (
	"context"
	"dagger/docker-client/internal/dagger"
)

func New(sock *dagger.Socket) *DockerClient {
	return &DockerClient{
		Sock: sock,
	}
}

type DockerClient struct {
	Sock *dagger.Socket
}

// Call "docker version" and return the output.
func (m *DockerClient) Version(ctx context.Context) (string, error) {
	return dag.Container().
		From("docker:27-cli").
		WithUnixSocket("/var/run/docker.sock", m.Sock).
		WithExec([]string{"docker", "version"}).
		Stdout(ctx)
}
