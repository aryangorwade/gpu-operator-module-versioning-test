# GPU Operator Module Versioning Experiment

This repository tests splitting the GPU Operator API into the nested
`github.com/NVIDIA/gpu-operator/api` Go module and releasing the operator and
API in lockstep. Every GPU Operator release supports exactly one API module
version.

## Version mapping

The API module stays at major version `0`. Its minor version is the operator
major and minor, each encoded as two digits and joined together. Its patch
version is copied from the operator:

```text
GPU Operator v26.3.2  =>  API version v0.2603.2  =>  Git tag api/v0.2603.2
             ^^ ^  ^                    ^^ ^  ^
             26 03 2                    26 03 2
```

The mapping can represent operator major and minor values from `0` through
`99`. Pre-release suffixes are not part of the API versioning contract.

## Current mapping

- GPU Operator version and tag: `v26.7.0`
- API module version required by the operator: `v0.2607.0`
- API module Git tag: `api/v0.2607.0`

Both tags must point to the same commit. The root module keeps a local
`replace github.com/NVIDIA/gpu-operator/api => ./api` directive for development,
while the required version records the API release supported by the operator.

## Maintainer release process

1. Ensure every direct dependency shared by `go.mod` and `api/go.mod` has the
   same version.
2. Create the normal operator release PR. In that PR, update the
   `github.com/NVIDIA/gpu-operator/api` requirement in the root `go.mod` to the
   version mapped from the upcoming operator version. Run `go mod vendor` and
   commit the resulting module metadata.
3. Merge the release PR.
4. Tag the merged commit with both release tags and push them together:

   ```bash
   git tag v26.7.0
   git tag api/v0.2607.0
   git push origin v26.7.0 api/v0.2607.0
   ```

Do not create the API tag before the release PR merges. Do not move either tag
after publishing it.

## CI validation

Release PR validation:

- verifies the operator version maps to the API version required by `go.mod`;
- verifies shared direct dependencies use identical versions;
- verifies vendor metadata is current through the normal module checks; and
- when the root API requirement changes, verifies the corresponding API tag
  has not already been published.

Post-tag validation:

- resolves either release tag to its required companion tag;
- verifies both tags point to the final release commit;
- builds the operator from that commit;
- verifies the executable records the mapped API dependency;
- verifies `go list -m github.com/NVIDIA/gpu-operator/api@<version>` resolves
  the published API module; and
- builds and runs an independent API consumer.

`go list -m` validates the API module version. The operator release version is
validated from its Git tag and `versions.mk`; the root module cannot be queried
at `v26.x.y` under Go's semantic import versioning rules without changing its
module path to include `/v26`.

## Local validation

Run the version mapping checks:

```bash
make validate-versioning
```

Run the local module and executable tests:

```bash
go test .
```

Published-tag and external-consumer tests run only in post-tag CI.
