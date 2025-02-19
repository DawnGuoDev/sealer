// Copyright © 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package buildah

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/containers/buildah/pkg/parse"

	"os"

	"github.com/sealerio/sealer/pkg/define/options"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/common/pkg/auth"
	"github.com/pkg/errors"
)

func (engine *Engine) Pull(opts *options.PullOptions) error {
	if len(opts.Image) == 0 {
		return errors.Errorf("an image name must be specified")
	}

	systemCxt := engine.SystemContext()
	store := engine.ImageStore()
	if err := auth.CheckAuthFile(systemCxt.AuthFilePath); err != nil {
		return err
	}

	// we need to new a systemContext instead of taking the systemContext of engine,
	// because pullOption does not export platform option
	newSystemCxt := systemContext()
	_os, arch, variant, err := parse.Platform(opts.Platform)
	if err != nil {
		return errors.Errorf("failed to init platform from %s: %v", opts.Platform, err)
	}
	newSystemCxt.OSChoice = _os
	newSystemCxt.ArchitectureChoice = arch
	newSystemCxt.VariantChoice = variant

	policy, ok := define.PolicyMap[opts.PullPolicy]
	if !ok {
		return fmt.Errorf("unsupported pull policy %q", opts.PullPolicy)
	}
	options := buildah.PullOptions{
		Store:         store,
		SystemContext: newSystemCxt,
		// consider export this option later
		AllTags:      false,
		ReportWriter: os.Stderr,
		MaxRetries:   maxPullPushRetries,
		RetryDelay:   pullPushRetryDelay,
		PullPolicy:   policy,
	}

	if opts.Quiet {
		options.ReportWriter = nil // Turns off logging output
	}

	id, err := buildah.Pull(getContext(), opts.Image, options)
	if err != nil {
		return err
	}
	logrus.Infof("%s", id)
	return nil
}
