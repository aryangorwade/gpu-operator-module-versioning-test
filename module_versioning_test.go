/*
Copyright NVIDIA CORPORATION

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gpuoperator_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

const (
	operatorModulePath = "github.com/NVIDIA/gpu-operator"
	apiModulePath      = "github.com/NVIDIA/gpu-operator/api"
	repositoryRemote   = "origin"
)

var (
	operatorVersion    string
	apiVersion         string
	operatorBinaryPath string
)

func TestMain(m *testing.M) {
	var err error
	operatorVersion, err = readOperatorVersion("versions.mk")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	apiVersion, err = operatorToAPI(operatorVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "gpu-operator-versioning-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	operatorBinaryPath = filepath.Join(tmpDir, "gpu-operator")
	ldflags := fmt.Sprintf("-X %s/internal/info.version=%s", operatorModulePath, operatorVersion)
	out, err := exec.Command(
		"go", "build", "-mod=vendor", "-ldflags", ldflags,
		"-o", operatorBinaryPath, "./cmd/gpu-operator",
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gpu-operator build failed: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestOperatorToAPIVersion(t *testing.T) {
	tests := map[string]string{
		"v26.3.2":  "v0.2603.2",
		"v26.7.0":  "v0.2607.0",
		"v26.10.4": "v0.2610.4",
	}
	for operator, expectedAPI := range tests {
		t.Run(operator, func(t *testing.T) {
			actualAPI, err := operatorToAPI(operator)
			if err != nil {
				t.Fatal(err)
			}
			if actualAPI != expectedAPI {
				t.Fatalf("operatorToAPI(%q) = %q, want %q", operator, actualAPI, expectedAPI)
			}
		})
	}
}

func TestRootRequiresMappedAPIVersion(t *testing.T) {
	requiredVersion, err := readRequiredAPIVersion("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if requiredVersion != apiVersion {
		t.Fatalf(
			"%s requires %s %s; %s maps to %s",
			operatorModulePath, apiModulePath, requiredVersion, operatorVersion, apiVersion,
		)
	}
}

func TestBinaryRecordsMappedAPIDependency(t *testing.T) {
	out, err := exec.Command("go", "version", "-m", operatorBinaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("go version -m failed: %v\n%s", err, out)
	}

	expected := fmt.Sprintf("dep\t%s\t%s", apiModulePath, apiVersion)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), expected) {
			return
		}
	}
	t.Fatalf("binary metadata does not contain %q:\n%s", expected, out)
}

func TestReleaseTagsPointToSameCommit(t *testing.T) {
	requireReleaseValidation(t)

	operatorTag := operatorVersion
	apiTag := "api/" + apiVersion
	operatorCommit := remoteTagCommit(t, operatorTag)
	apiCommit := remoteTagCommit(t, apiTag)
	if operatorCommit != apiCommit {
		t.Fatalf(
			"%s points to %s, but %s points to %s",
			operatorTag, operatorCommit, apiTag, apiCommit,
		)
	}
}

func TestPublishedAPIModuleVersion(t *testing.T) {
	requireReleaseValidation(t)

	consumerDir := t.TempDir()
	writeFile(t, filepath.Join(consumerDir, "go.mod"), "module example.com/version-query\n\ngo 1.26.3\n")

	cmd := exec.Command("go", "list", "-m", "-json", apiModulePath+"@"+apiVersion)
	cmd.Dir = consumerDir
	cmd.Env = releaseGoEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -m failed: %v\n%s", err, out)
	}

	var module struct {
		Path    string
		Version string
	}
	if err := json.Unmarshal(out, &module); err != nil {
		t.Fatalf("decode go list output: %v\n%s", err, out)
	}
	if module.Path != apiModulePath || module.Version != apiVersion {
		t.Fatalf(
			"go list returned %s@%s, want %s@%s",
			module.Path, module.Version, apiModulePath, apiVersion,
		)
	}
}

func TestPublishedAPIModuleCanBeConsumed(t *testing.T) {
	requireReleaseValidation(t)

	consumerDir := t.TempDir()
	writeFile(t, filepath.Join(consumerDir, "go.mod"), fmt.Sprintf(`module example.com/gpu-operator-api-consumer

go 1.26.3

require %s %s
`, apiModulePath, apiVersion))
	writeFile(t, filepath.Join(consumerDir, "main.go"), `package main

import (
	"fmt"

	nvidiav1alpha1 "github.com/NVIDIA/gpu-operator/api/nvidia/v1alpha1"
)

func main() {
	driver := nvidiav1alpha1.NVIDIADriver{}
	driver.Name = "consumer-test"
	if err := driver.ValidateNodeSelector(); err != nil {
		panic(err)
	}
	fmt.Print(nvidiav1alpha1.NVIDIADriverOwnerLabel)
}
`)

	cmd := exec.Command("go", "run", "-mod=mod", ".")
	cmd.Dir = consumerDir
	cmd.Env = releaseGoEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("API consumer failed to build and run: %v\n%s", err, stderr.String())
	}
	if got, want := stdout.String(), "nvidia.com/gpu-operator.driver.owner"; got != want {
		t.Fatalf("unexpected API consumer output: got %q, want %q", got, want)
	}
}

func operatorToAPI(operator string) (string, error) {
	match := regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`).FindStringSubmatch(operator)
	if match == nil {
		return "", fmt.Errorf("operator version must look like v26.3.2: %s", operator)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	if major > 99 || minor > 99 {
		return "", fmt.Errorf("operator major and minor versions must each fit in two digits")
	}
	return fmt.Sprintf("v0.%02d%02d.%d", major, minor, patch), nil
}

func readOperatorVersion(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`(?m)^VERSION\s*\?=\s*(v[^\s]+)\s*$`).FindSubmatch(contents)
	if match == nil {
		return "", fmt.Errorf("could not read VERSION from %s", path)
	}
	return string(match[1]), nil
}

func readRequiredAPIVersion(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	file, err := modfile.Parse(path, contents, nil)
	if err != nil {
		return "", err
	}
	for _, requirement := range file.Require {
		if requirement.Mod.Path == apiModulePath {
			return requirement.Mod.Version, nil
		}
	}
	return "", fmt.Errorf("%s does not require %s", path, apiModulePath)
}

func remoteTagCommit(t *testing.T, tag string) string {
	t.Helper()
	out, err := exec.Command(
		"git", "ls-remote", "--tags", repositoryRemote,
		"refs/tags/"+tag, "refs/tags/"+tag+"^{}",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to query tag %s: %v\n%s", tag, err, out)
	}

	var commit string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		commit = fields[0]
		if strings.HasSuffix(fields[1], "^{}") {
			return commit
		}
	}
	if commit == "" {
		t.Fatalf("release tag %s is not published", tag)
	}
	return commit
}

func requireReleaseValidation(t *testing.T) {
	t.Helper()
	if os.Getenv("GPU_OPERATOR_RELEASE_VALIDATION") != "1" {
		t.Skip("published release validation only runs after tags are pushed")
	}
}

func releaseGoEnv() []string {
	env := append([]string{}, os.Environ()...)
	return append(env, "GOPROXY=direct", "GOSUMDB=off")
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
