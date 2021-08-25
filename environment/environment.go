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

func GetEnvironment() *Environment {
	return &Environment{
		EnvVars:    os.Environ(),
		GitCommit:  gitCommit,
		GoMaxProcs: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
	}
}
