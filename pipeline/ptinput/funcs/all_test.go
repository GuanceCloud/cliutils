// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/platypus/pkg/engine"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
)

func NewTestingRunner(script string) (*runtime.Script, error) {
	name := "default.p"
	ret1, ret2 := engine.ParseScript(map[string]string{
		"default.p": script,
	},
		FuncsMap, FuncsCheckMap,
	)
	if len(ret1) > 0 {
		return ret1[name], nil
	}
	if len(ret2) > 0 {
		return nil, ret2[name]
	}
	return nil, fmt.Errorf("parser func error")
}

func NewTestingRunner2(scripts map[string]string) (map[string]*runtime.Script, map[string]error) {
	return engine.ParseScript(scripts, FuncsMap, FuncsCheckMap)
}

func runScript(proc *runtime.Script, pt ptinput.PlInputPt, fn ...runtime.TaskFn) *errchain.PlError {
	return proc.Run(pt, nil, fn...)
}
