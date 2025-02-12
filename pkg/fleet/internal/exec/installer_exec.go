// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package exec provides an implementation of the Installer interface that uses the installer binary.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/fleet/internal/paths"
	"github.com/DataDog/datadog-agent/pkg/util/log"

	"github.com/DataDog/datadog-agent/pkg/fleet/installer/env"
	installerErrors "github.com/DataDog/datadog-agent/pkg/fleet/installer/errors"
	"github.com/DataDog/datadog-agent/pkg/fleet/installer/repository"
	"github.com/DataDog/datadog-agent/pkg/fleet/telemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// InstallerExec is an implementation of the Installer interface that uses the installer binary.
type InstallerExec struct {
	env              *env.Env
	installerBinPath string
}

// NewInstallerExec returns a new InstallerExec.
func NewInstallerExec(env *env.Env, installerBinPath string) *InstallerExec {
	return &InstallerExec{
		env:              env,
		installerBinPath: installerBinPath,
	}
}

type installerCmd struct {
	*exec.Cmd
	span tracer.Span
	ctx  context.Context
}

func (i *InstallerExec) newInstallerCmd(ctx context.Context, command string, args ...string) *installerCmd {
	env := i.env.ToEnv()
	span, ctx := tracer.StartSpanFromContext(ctx, fmt.Sprintf("installer.%s", command))
	span.SetTag("args", args)
	cmd := exec.CommandContext(ctx, i.installerBinPath, append([]string{command}, args...)...)
	env = append(os.Environ(), env...)
	if runtime.GOOS != "windows" {
		// os.Interrupt is not support on Windows
		// It gives " run failed: exec: canceling Cmd: not supported by windows"
		cmd.Cancel = func() error {
			return cmd.Process.Signal(os.Interrupt)
		}
	}
	env = append(env, telemetry.EnvFromSpanContext(span.Context())...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &installerCmd{
		Cmd:  cmd,
		span: span,
		ctx:  ctx,
	}
}

// Install installs a package.
func (i *InstallerExec) Install(ctx context.Context, url string, _ []string) (err error) {
	cmd := i.newInstallerCmd(ctx, "install", url)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// Remove removes a package.
func (i *InstallerExec) Remove(ctx context.Context, pkg string) (err error) {
	cmd := i.newInstallerCmd(ctx, "remove", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// Purge - noop, must be called by the package manager on uninstall.
func (i *InstallerExec) Purge(_ context.Context) {
	panic("don't call Purge directly")
}

// InstallExperiment installs an experiment.
func (i *InstallerExec) InstallExperiment(ctx context.Context, url string) (err error) {
	cmd := i.newInstallerCmd(ctx, "install-experiment", url)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// RemoveExperiment removes an experiment.
func (i *InstallerExec) RemoveExperiment(ctx context.Context, pkg string) (err error) {
	cmd := i.newInstallerCmd(ctx, "remove-experiment", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// PromoteExperiment promotes an experiment to stable.
func (i *InstallerExec) PromoteExperiment(ctx context.Context, pkg string) (err error) {
	cmd := i.newInstallerCmd(ctx, "promote-experiment", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// InstallConfigExperiment installs an experiment.
func (i *InstallerExec) InstallConfigExperiment(ctx context.Context, url string, version string) (err error) {
	cmd := i.newInstallerCmd(ctx, "install-config-experiment", url, version)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// RemoveConfigExperiment removes an experiment.
func (i *InstallerExec) RemoveConfigExperiment(ctx context.Context, pkg string) (err error) {
	cmd := i.newInstallerCmd(ctx, "remove-config-experiment", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// PromoteConfigExperiment promotes an experiment to stable.
func (i *InstallerExec) PromoteConfigExperiment(ctx context.Context, pkg string) (err error) {
	cmd := i.newInstallerCmd(ctx, "promote-config-experiment", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// GarbageCollect runs the garbage collector.
func (i *InstallerExec) GarbageCollect(ctx context.Context) (err error) {
	cmd := i.newInstallerCmd(ctx, "garbage-collect")
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// InstrumentAPMInjector instruments the APM auto-injector.
func (i *InstallerExec) InstrumentAPMInjector(ctx context.Context, method string) (err error) {
	cmd := i.newInstallerCmd(ctx, "apm instrument", method)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// UninstrumentAPMInjector uninstruments the APM auto-injector.
func (i *InstallerExec) UninstrumentAPMInjector(ctx context.Context, method string) (err error) {
	cmd := i.newInstallerCmd(ctx, "apm uninstrument", method)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	return cmd.Run()
}

// IsInstalled checks if a package is installed.
func (i *InstallerExec) IsInstalled(ctx context.Context, pkg string) (_ bool, err error) {
	cmd := i.newInstallerCmd(ctx, "is-installed", pkg)
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	err = cmd.Run()
	if err != nil && cmd.ProcessState.ExitCode() == 10 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DefaultPackages returns the default packages to install.
func (i *InstallerExec) DefaultPackages(ctx context.Context) (_ []string, err error) {
	cmd := i.newInstallerCmd(ctx, "default-packages")
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error running default-packages: %w\n%s", err, stderr.String())
	}
	var defaultPackages []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" {
			continue
		}
		defaultPackages = append(defaultPackages, line)
	}
	return defaultPackages, nil
}

// Setup runs the setup command.
func (i *InstallerExec) Setup(ctx context.Context) (err error) {
	cmd := i.newInstallerCmd(ctx, "setup")
	defer func() { cmd.span.Finish(tracer.WithError(err)) }()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error running setup: %w\n%s", err, stderr.String())
	}
	return nil
}

// AvailableDiskSpace returns the available disk space.
func (i *InstallerExec) AvailableDiskSpace() (uint64, error) {
	repositories := repository.NewRepositories(paths.PackagesPath, paths.LocksPath)
	return repositories.AvailableDiskSpace()
}

// State returns the state of a package.
func (i *InstallerExec) State(pkg string) (repository.State, error) {
	repositories := repository.NewRepositories(paths.PackagesPath, paths.LocksPath)
	return repositories.Get(pkg).GetState()
}

// States returns the states of all packages.
func (i *InstallerExec) States() (map[string]repository.State, error) {
	repositories := repository.NewRepositories(paths.PackagesPath, paths.LocksPath)
	states, err := repositories.GetStates()
	log.Debugf("repositories states: %v", states)
	return states, err
}

// ConfigState returns the state of a package's configuration.
func (i *InstallerExec) ConfigState(pkg string) (repository.State, error) {
	repositories := repository.NewRepositories(paths.ConfigsPath, paths.LocksPath)
	return repositories.Get(pkg).GetState()
}

// ConfigStates returns the states of all packages' configurations.
func (i *InstallerExec) ConfigStates() (map[string]repository.State, error) {
	repositories := repository.NewRepositories(paths.ConfigsPath, paths.LocksPath)
	states, err := repositories.GetStates()
	log.Debugf("config repositories states: %v", states)
	return states, err
}

// Close cleans up any resources.
func (i *InstallerExec) Close() error {
	return nil
}

func (iCmd *installerCmd) Run() error {
	var errBuf bytes.Buffer
	iCmd.Stderr = &errBuf
	err := iCmd.Cmd.Run()
	if err == nil {
		return nil
	}

	if len(errBuf.Bytes()) == 0 {
		return fmt.Errorf("run failed: %w", err)
	}

	installerError := installerErrors.FromJSON(strings.TrimSpace(errBuf.String()))
	return fmt.Errorf("run failed: %v \n%s", installerError, err.Error())
}
