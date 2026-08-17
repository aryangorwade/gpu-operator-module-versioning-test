#!/usr/bin/env python3

# Copyright NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Validate the lockstep GPU Operator and API module versions."""

import argparse
import json
import pathlib
import re
import sys


API_MODULE = "github.com/NVIDIA/gpu-operator/api"
OPERATOR_VERSION_PATTERN = re.compile(r"^v([0-9]+)\.([0-9]+)\.([0-9]+)$")
API_VERSION_PATTERN = re.compile(r"^v0\.([0-9]{4})\.([0-9]+)$")


def operator_to_api(operator_version):
    match = OPERATOR_VERSION_PATTERN.fullmatch(operator_version)
    if not match:
        raise ValueError(
            f"operator version must look like v26.3.2: {operator_version}"
        )

    major, minor, patch = (int(part) for part in match.groups())
    if major > 99 or minor > 99:
        raise ValueError(
            "operator major and minor versions must each fit in two digits"
        )

    return f"v0.{major:02d}{minor:02d}.{patch}"


def api_to_operator(api_version):
    match = API_VERSION_PATTERN.fullmatch(api_version)
    if not match:
        raise ValueError(
            f"API version must look like v0.2603.2: {api_version}"
        )

    encoded_minor, patch = match.groups()
    major = int(encoded_minor[:2])
    minor = int(encoded_minor[2:])
    return f"v{major}.{minor}.{int(patch)}"


def read_operator_version(path):
    contents = pathlib.Path(path).read_text()
    match = re.search(r"(?m)^VERSION\s*\?=\s*(v[^\s]+)\s*$", contents)
    if not match:
        raise ValueError(f"could not read VERSION from {path}")
    return match.group(1)


def read_required_api_version(path):
    contents = pathlib.Path(path).read_text()
    pattern = re.compile(
        rf"(?m)^\s*(?:require\s+)?{re.escape(API_MODULE)}\s+"
        rf"(v[^\s]+)(?:\s+//.*)?$"
    )
    match = pattern.search(contents)
    if not match:
        raise ValueError(f"could not read {API_MODULE} requirement from {path}")
    return match.group(1)


def resolve_tag(tag):
    if tag.startswith("api/"):
        api_version = tag.removeprefix("api/")
        operator_version = api_to_operator(api_version)
    else:
        operator_version = tag
        api_version = operator_to_api(operator_version)

    return {
        "operator_tag": operator_version,
        "api_version": api_version,
        "api_tag": f"api/{api_version}",
    }


def write_github_output(path, values):
    with pathlib.Path(path).open("a") as output:
        for key, value in values.items():
            output.write(f"{key}={value}\n")


def command_expected_api(args):
    print(operator_to_api(args.operator_version))


def command_required_api(args):
    print(read_required_api_version(args.go_mod))


def command_resolve_tag(args):
    values = resolve_tag(args.tag)
    if args.github_output:
        write_github_output(args.github_output, values)
    else:
        print(json.dumps(values, sort_keys=True))


def command_validate(args):
    operator_version = read_operator_version(args.versions_file)
    required_api_version = read_required_api_version(args.go_mod)
    expected_api_version = operator_to_api(operator_version)

    if required_api_version != expected_api_version:
        raise ValueError(
            f"{args.go_mod} requires {API_MODULE} {required_api_version}; "
            f"{operator_version} requires {expected_api_version}"
        )

    print(
        f"Verified {operator_version} -> {API_MODULE} {required_api_version} "
        f"(tag api/{required_api_version})"
    )


def parse_args():
    parser = argparse.ArgumentParser(
        description="Validate lockstep GPU Operator and API module versions."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    expected_api = subparsers.add_parser(
        "expected-api", help="derive an API version from an operator version"
    )
    expected_api.add_argument("operator_version")
    expected_api.set_defaults(func=command_expected_api)

    required_api = subparsers.add_parser(
        "required-api", help="read the API version required by a go.mod"
    )
    required_api.add_argument("--go-mod", default="go.mod")
    required_api.set_defaults(func=command_required_api)

    resolve_tag_parser = subparsers.add_parser(
        "resolve-tag", help="resolve either release tag to its lockstep pair"
    )
    resolve_tag_parser.add_argument("tag")
    resolve_tag_parser.add_argument("--github-output")
    resolve_tag_parser.set_defaults(func=command_resolve_tag)

    validate = subparsers.add_parser(
        "validate", help="validate repository release metadata"
    )
    validate.add_argument("--versions-file", default="versions.mk")
    validate.add_argument("--go-mod", default="go.mod")
    validate.set_defaults(func=command_validate)

    return parser.parse_args()


def main():
    args = parse_args()
    try:
        args.func(args)
    except (OSError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
