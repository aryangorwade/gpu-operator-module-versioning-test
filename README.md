# GPU Operator Module Versioning Experiment

This repository tests splitting the GPU Operator API into the nested
`github.com/NVIDIA/gpu-operator/api` Go module. The root module requires API
version `v0.8.0`, uses `./api` for local development, and publishes the nested
module with Git tag `api/v0.8.0`.

## Tests

`module_versioning_test.go` verifies that:

- the GPU Operator binary builds;
- the root and API tags exist locally and on the remote;
- the forked root module records a pseudo-version because its declared module
  path differs from this repository;
- binary build metadata records the API as `v0.8.0`, not a pseudo-version;
- the operator dependency graph imports the released API module; and
- an external consumer can build and use the API from its tagged contents.

The `Verify Shared Dependency Versions` workflow ensures that direct
dependencies shared by the root and API modules use identical versions.

## Running

Requires Go 1.26.3 and access to the repository's `origin` remote.

```bash
go test -v .
```
