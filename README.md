# GPU Operator Module Versioning Experiment

This repository tests splitting the GPU Operator API into the nested
`github.com/NVIDIA/gpu-operator/api` Go module and releasing it in lockstep with
the GPU Operator.

## Version mapping

The API module remains at major version `0`. Its minor version joins the
two-digit operator major and minor versions, and its patch version matches the
operator patch version:

`v<operator-major>.<operator-minor>.<operator-patch>` becomes
`api/v0.<operator-major><operator-minor>.<operator-patch>`.

The current release mapping is:

- GPU Operator: `v26.3.3`
- API module version: `v0.2603.3`
- API Git tag: `api/v0.2603.3`

Each GPU Operator release supports exactly one API module version.

## Release process

1. Ensure dependencies shared by the root and API modules use the same
   versions.
2. In the release PR, update the API requirement in the root `go.mod` to the
   upcoming mapped version.
3. Merge the release PR.
4. Tag the merged commit with both the operator tag and the mapped API tag, then
   push both tags. For example:

   ```bash
   git tag v26.7.0
   git tag api/v0.2607.0
   git push origin v26.7.0 api/v0.2607.0
   ```

Release PR CI checks that shared direct dependencies remain synchronized and
that the new API version has not already been tagged. Post-tag CI confirms that
both tags point to the same commit, the root module requires the expected API
version, the operator builds, and the API version is reported by `go list -m`.

## Tests

`module_versioning_test.go` verifies the lockstep tags, binary module metadata,
operator dependency graph, and external API consumption.

Running the test requires Go 1.26.3 and access to the repository's `origin`
remote:

```bash
go test -v .
```
