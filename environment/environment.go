package environment

import (
	"os"
	"runtime"
)

var gitCommit string

type Environment struct {
	EnvVars    []string `json:"envVars"`
	GitCommit  string   `json:"gitCommit"`
	GoMaxProcs int      `json:"goMaxProcs"`
	GoVersion  string   `json:"goVersion"`
}

var environment *Environment

func GetEnvironment() *Environment {
	return environment
}

func init() {
	environment = &Environment{
		EnvVars:    os.Environ(),
		GitCommit:  gitCommit,
		GoMaxProcs: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
	}
}
