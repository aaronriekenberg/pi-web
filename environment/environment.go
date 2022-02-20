package environment

import (
	"os"
	"runtime"
	"sort"
)

var gitCommit string

type Environment struct {
	EnvVars    []string `json:"envVars"`
	GitCommit  string   `json:"gitCommit"`
	GoMaxProcs int      `json:"goMaxProcs"`
	GoVersion  string   `json:"goVersion"`
}

func (environment *Environment) deepCopy() *Environment {
	envVarsCopy := make([]string, len(environment.EnvVars))
	copy(envVarsCopy, environment.EnvVars)

	return &Environment{
		EnvVars:    envVarsCopy,
		GitCommit:  environment.GitCommit,
		GoMaxProcs: environment.GoMaxProcs,
		GoVersion:  environment.GoVersion,
	}
}

func GetEnvironment() *Environment {
	return environmentInstance.deepCopy()
}

var environmentInstance *Environment

func init() {
	envVars := os.Environ()
	sort.Strings(envVars)

	environmentInstance = &Environment{
		EnvVars:    envVars,
		GitCommit:  gitCommit,
		GoMaxProcs: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
	}
}
