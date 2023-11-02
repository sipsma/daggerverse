package main

import (
	"context"
)

type Yamlinvaders struct{}

func (m *Yamlinvaders) Play(ctx context.Context) (*Container, error) {
	repo := dag.
		Git("https://github.com/macdice/ascii-invaders.git").
		Branch("master").
		Tree()

	return dag.Container().From("debian:buster").
		WithExec([]string{"apt", "update"}).
		WithExec([]string{"apt", "install", "-y", "build-essential", "libncursesw5-dev", "git"}).
		WithMountedDirectory("/src", repo).
		WithMountedFile("/dagger.patch", dag.Host().File("./dagger.patch")).
		WithWorkdir("/src").
		WithExec([]string{"git", "apply", "/dagger.patch"}).
		WithExec([]string{"make"}).
		WithEntrypoint([]string{"./ascii_invaders"}).
		Sync(ctx)
}
