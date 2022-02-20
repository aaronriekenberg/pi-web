package environment

import (
	"encoding/json"
	"os"
	"runtime"
)

var gitCommit string

type environmentJSON struct {
	EnvVars    []string `json:"envVars"`
	GitCommit  string   `json:"gitCommit"`
	GoMaxProcs int      `json:"goMaxProcs"`
	GoVersion  string   `json:"goVersion"`
}

type environment struct {
	envVars    []string
	gitCommit  string
	goMaxProcs int
	goVersion  string
}

type Environment interface {
	EnvVars() []string

	GitCommit() string

	GoMaxProcs() int

	GoVersion() string

	MarshalJSON() ([]byte, error)
}

func GetEnvironment() Environment {
	return environmentInstance
}

func (environment *environment) EnvVars() []string {
	envVarsCopy := make([]string, len(environment.envVars))
	copy(envVarsCopy, environment.envVars)
	return envVarsCopy
}

func (environment *environment) GitCommit() string {
	return environment.gitCommit
}

func (environment *environment) GoMaxProcs() int {
	return environment.goMaxProcs
}

func (environment *environment) GoVersion() string {
	return environment.goVersion
}

func (environment *environment) MarshalJSON() ([]byte, error) {
	return json.Marshal(environmentJSON{
		EnvVars:    environment.envVars,
		GitCommit:  environment.gitCommit,
		GoMaxProcs: environment.goMaxProcs,
		GoVersion:  environment.goVersion,
	})
}

var environmentInstance *environment

func init() {
	environmentInstance = &environment{
		envVars:    os.Environ(),
		gitCommit:  gitCommit,
		goMaxProcs: runtime.GOMAXPROCS(0),
		goVersion:  runtime.Version(),
	}
}
