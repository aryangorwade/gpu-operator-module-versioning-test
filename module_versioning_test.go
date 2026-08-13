package gpuoperator_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	operatorModulePath = "github.com/NVIDIA/gpu-operator"
	apiModulePath      = "github.com/NVIDIA/gpu-operator/api"
	operatorTag        = "v26.3.3"
	apiTag             = "api/v0.8.0"
	apiVersion         = "v0.8.0"
	repositoryRemote   = "origin"
)

var operatorBinaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "gpu-operator-versioning-test-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	operatorBinaryPath = filepath.Join(tmpDir, "gpu-operator")
	out, err := exec.Command("go", "build", "-o", operatorBinaryPath, "./cmd/gpu-operator").CombinedOutput()
	if err != nil {
		os.Stderr.WriteString("gpu-operator build failed:\n" + string(out) + "\n")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestBinaryExists(t *testing.T) {
	if _, err := os.Stat(operatorBinaryPath); err != nil {
		t.Fatalf("gpu-operator binary not found at %s: %v", operatorBinaryPath, err)
	}
}

func TestRootModuleTagExists(t *testing.T) {
	assertPublishedTag(t, operatorTag)
}

func TestAPIModuleTagExists(t *testing.T) {
	assertPublishedTag(t, apiTag)
}

func TestVersionMetadata(t *testing.T) {
	out, err := exec.Command("go", "version", "-m", operatorBinaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("go version -m failed: %v\n%s", err, out)
	}
	output := string(out)
	rootModuleLine := findBuildInfoLine(t, output, "mod\t"+operatorModulePath+"\t")
	apiDependencyLine := findBuildInfoLine(t, output, "dep\t"+apiModulePath+"\t")

	t.Run("RootModuleUsesPseudoVersion", func(t *testing.T) {
		pseudoVersion := regexp.MustCompile(`v\d+\.\d+\.\d+-\d{14}-[0-9a-f]{12}`)
		if !pseudoVersion.MatchString(rootModuleLine) {
			t.Errorf("expected incompatible operator tag %s to produce a pseudo-version, got %q", operatorTag, rootModuleLine)
		}
	})

	t.Run("APIModuleIsDepEntry", func(t *testing.T) {
		if apiDependencyLine == "" {
			t.Errorf("expected API module as a dependency entry:\n%s", output)
		}
	})

	t.Run("APIModuleCleanSemver", func(t *testing.T) {
		if !strings.Contains(apiDependencyLine, "\t"+apiVersion) {
			t.Errorf("expected API module at %s, got %q", apiVersion, apiDependencyLine)
		}
	})

	t.Run("APIModuleNoPseudoVersion", func(t *testing.T) {
		pseudoVersion := regexp.MustCompile(`v\d+\.\d+\.\d+-\d{14}-[0-9a-f]{12}`)
		if pseudoVersion.MatchString(apiDependencyLine) {
			t.Errorf("API dependency contains a pseudo-version: %s", apiDependencyLine)
		}
	})
}

func TestOperatorImportsReleasedAPIModule(t *testing.T) {
	out, err := exec.Command(
		"go", "list", "-mod=mod", "-deps",
		"-f", `{{with .Module}}{{.Path}}@{{.Version}}{{end}}`,
		"./cmd/gpu-operator",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}

	expected := apiModulePath + "@" + apiVersion
	for _, line := range strings.Split(string(out), "\n") {
		if line == expected {
			return
		}
	}
	t.Fatalf("operator dependency graph does not contain %s:\n%s", expected, out)
}

func TestReleasedAPIModuleCanBeImported(t *testing.T) {
	localTagCommit(t, apiTag)

	apiDir := extractTaggedAPI(t)

	consumerDir := t.TempDir()
	writeFile(t, filepath.Join(consumerDir, "go.mod"), `module example.com/gpu-operator-api-consumer

go 1.26.3

require `+apiModulePath+` `+apiVersion+`

replace `+apiModulePath+` => `+apiDir+`
`)
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

func extractTaggedAPI(t *testing.T) string {
	t.Helper()
	modulePrefix := gitOutput(t, "rev-parse", "--show-prefix")
	apiRepositoryPath := filepath.ToSlash(filepath.Join(modulePrefix, "api"))
	archive, err := exec.Command("git", "archive", apiTag, "--", apiRepositoryPath).Output()
	if err != nil {
		t.Fatalf("failed to archive API module from %s: %v", apiTag, err)
	}

	apiDir := t.TempDir()
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeXHeader || header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		relativePath, err := filepath.Rel(filepath.FromSlash(apiRepositoryPath), filepath.FromSlash(header.Name))
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			t.Fatalf("invalid API archive path %q", header.Name)
		}
		destination := filepath.Join(apiDir, relativePath)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(file, reader); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	return apiDir
}

func assertPublishedTag(t *testing.T, tag string) {
	t.Helper()
	localCommit := localTagCommit(t, tag)

	out, err := exec.Command("git", "ls-remote", "--tags", repositoryRemote, "refs/tags/"+tag).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to query published tag %s: %v\n%s", tag, err, out)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		t.Fatalf("expected published tag %s, got %q", tag, out)
	}
	if fields[0] != localCommit {
		t.Fatalf("published tag %s points to %s, want %s", tag, fields[0], localCommit)
	}
}

func localTagCommit(t *testing.T, tag string) string {
	t.Helper()
	return gitOutput(t, "rev-list", "-n", "1", tag)
}

func findBuildInfoLine(t *testing.T, output, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func gitOutput(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
