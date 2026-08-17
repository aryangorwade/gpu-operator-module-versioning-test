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

import pathlib
import tempfile
import unittest

import versioning


class VersioningTest(unittest.TestCase):
    def test_operator_to_api(self):
        cases = {
            "v26.3.2": "v0.2603.2",
            "v26.7.0": "v0.2607.0",
            "v26.10.4": "v0.2610.4",
        }
        for operator_version, api_version in cases.items():
            with self.subTest(operator_version=operator_version):
                self.assertEqual(
                    versioning.operator_to_api(operator_version), api_version
                )

    def test_api_to_operator(self):
        self.assertEqual(
            versioning.api_to_operator("v0.2603.2"),
            "v26.3.2",
        )

    def test_rejects_unrepresentable_operator_version(self):
        with self.assertRaises(ValueError):
            versioning.operator_to_api("v26.100.0")

    def test_resolve_tag(self):
        expected = {
            "operator_tag": "v26.7.0",
            "api_version": "v0.2607.0",
            "api_tag": "api/v0.2607.0",
        }
        self.assertEqual(versioning.resolve_tag("v26.7.0"), expected)
        self.assertEqual(versioning.resolve_tag("api/v0.2607.0"), expected)

    def test_reads_repository_versions(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = pathlib.Path(directory)
            versions_file = directory / "versions.mk"
            go_mod = directory / "go.mod"
            versions_file.write_text("VERSION ?= v26.7.0\n")
            go_mod.write_text(
                "module example.com/operator\n\n"
                "require github.com/NVIDIA/gpu-operator/api v0.2607.0\n"
            )

            self.assertEqual(
                versioning.read_operator_version(versions_file), "v26.7.0"
            )
            self.assertEqual(
                versioning.read_required_api_version(go_mod), "v0.2607.0"
            )


if __name__ == "__main__":
    unittest.main()
