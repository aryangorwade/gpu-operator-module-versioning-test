## Description

<!-- Brief description of the change, including context or motivation -->

## Checklist

- [ ] No secrets, sensitive information, or unrelated changes
- [ ] Lint checks passing (`make lint`)
- [ ] Generated assets in-sync (`make validate-generated-assets`)
- [ ] Go mod artifacts in-sync (`make validate-modules`)
- [ ] Test cases are added for new code paths

## Release PR checklist

Complete these items when preparing an operator release:

- [ ] `versions.mk` contains the upcoming operator version
- [ ] Root `go.mod` requires the mapped, unpublished API version
- [ ] Shared direct dependencies match between root and API modules
- [ ] Vendor metadata was regenerated
- [ ] After merge, the operator and API tags will be pushed on the same commit

## Testing

<!-- How was this tested? e.g., unit tests, manual testing on cluster -->

