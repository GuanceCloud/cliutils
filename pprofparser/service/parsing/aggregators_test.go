// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package parsing

import (
	"fmt"
	"testing"

	"github.com/GuanceCloud/cliutils/testutil"
)

func TestCutGoFuncName(t *testing.T) {
	fmt.Println(cutGoFuncName("gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.(*tracer).StartSpan"))
	fmt.Println(cutGoFuncName("bytes.(*Buffer).Write"))
	fmt.Println(cutGoFuncName("main.Fibonacci"))
}

func TestGetDDDotnetMethodName(t *testing.T) {
	fnName := "|lm:System.Private.CoreLib |ns:System |ct:StartupHookProvider |fn:ProcessStartupHooks"

	method := getDDDotnetMethodName(fnName)

	testutil.Equals(t, "StartupHookProvider.ProcessStartupHooks", method)
}

func TestGetDDDotnetField(t *testing.T) {
	fnName := "|lm:System.Private.CoreLib |ns:System |ct:StartupHookProvider |fn:ProcessStartupHooks |lib:Standard"
	assembly := getDDDotnetField(fnName, AssemblyTag)
	namespace := getDDDotnetField(fnName, NamespaceTag)
	className := getDDDotnetField(fnName, ClassTag)
	methodName := getDDDotnetField(fnName, MethodTag)
	libName := getDDDotnetField(fnName, "|lib:")
	testutil.Equals(t, "System.Private.CoreLib", assembly)
	testutil.Equals(t, "System", namespace)
	testutil.Equals(t, "StartupHookProvider", className)
	testutil.Equals(t, "ProcessStartupHooks", methodName)
	testutil.Equals(t, "Standard", libName)
}
