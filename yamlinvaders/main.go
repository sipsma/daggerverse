package main

import (
	"context"
)

type Yamlinvaders struct{}

// A silly TUI game that let's you destroy YAML. Press space to start on menu, arrow keys to move and space to shoot.
func (m *Yamlinvaders) Play(ctx context.Context) *Container {
	repo := dag.
		Git("https://github.com/macdice/ascii-invaders.git").
		Branch("master").
		Tree()

	return dag.Container().From("debian:buster").
		WithExec([]string{"apt", "update"}).
		WithExec([]string{"apt", "install", "-y", "build-essential", "libncursesw5-dev", "git"}).
		WithMountedDirectory("/src", repo).
		WithMountedFile("/dagger.patch", dag.CurrentModule().Source().File("./dagger.patch")).
		WithWorkdir("/src").
		WithExec([]string{"git", "apply", "/dagger.patch"}).
		WithExec([]string{"make"}).
		WithDefaultShell([]string{"./ascii_invaders"})
}
