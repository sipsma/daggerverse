package main

import (
	"context"
)

type Example struct{}

const defaultNodeVersion = "16"

func (m *Example) AppContainer(
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) *Container {
	return dag.Container().From("cgr.dev/chainguard/nginx:latest").
		WithDirectory("/usr/share/nginx/html", m.Build(srcDir, nodeVersion)).
		WithExposedPort(8080)
}

// TODO: remove this after https://github.com/dagger/dagger/pull/6039 merged+released
func (m *Example) Service(
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) *Service {
	return m.AppContainer(srcDir, nodeVersion).AsService()
}

func (m *Example) Debug(
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) *Container {
	return m.buildBase(srcDir, nodeVersion).Container().
		WithEntrypoint([]string{"sh"}).
		WithDefaultArgs()
}

func (m *Example) Build(
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) *Directory {
	return m.buildBase(srcDir, nodeVersion).Build().Container().Directory("./build")
}

func (m *Example) Test(
	ctx context.Context,
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) (string, error) {
	return m.buildBase(srcDir, nodeVersion).
		Run([]string{"test", "--", "--watchAll=false"}).
		Stderr(ctx)
}

func (m *Example) PublishContainer(
	ctx context.Context,
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) (string, error) {
	return dag.Ttlsh().Publish(ctx, m.AppContainer(srcDir, nodeVersion))
}

func (m *Example) buildBase(
	srcDir Optional[*Directory],
	nodeVersion Optional[string],
) *Node {
	return dag.Node().
		WithVersion(nodeVersion.GetOr(defaultNodeVersion)).
		WithNpm().
		WithSource(srcDir.GetOr(dag.Git("https://github.com/dagger/hello-dagger.git").Branch("main").Tree())).
		Install(nil)
}
