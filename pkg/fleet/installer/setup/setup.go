// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package setup contains the different setup scenarios
package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/datadog-agent/pkg/fleet/installer/env"
	"github.com/DataDog/datadog-agent/pkg/fleet/installer/setup/djm"
	"github.com/DataDog/datadog-agent/pkg/fleet/internal/exec"
	"github.com/DataDog/datadog-agent/pkg/fleet/internal/paths"
)

// Setup is the main function to resolve packages to install and install them
func Setup(ctx context.Context, env *env.Env) error {
	if os.Getenv("DD_DJM_FLAVOR") == "databricks" {
		return djm.SetupDatabricks(ctx, env)
	}

	return defaultSetup(ctx, env)
}

func defaultSetup(ctx context.Context, env *env.Env) error {
	cmd := exec.NewInstallerExec(env, paths.StableInstallerPath)
	defaultPackages, err := cmd.DefaultPackages(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default packages: %w", err)
	}
	for _, url := range defaultPackages {
		err = cmd.Install(ctx, url, nil)
		if err != nil {
			return fmt.Errorf("failed to install package %s: %w", url, err)
		}
	}
	return nil
}
