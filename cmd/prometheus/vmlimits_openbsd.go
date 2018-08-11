/*
Sniperkit-Bot
- Date: 2018-08-11 23:49:10.542900072 +0200 CEST m=+0.207767820
- Status: analyzed
*/

// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build openbsd

package main

import (
	"syscall"
)

// VmLimits returns the soft and hard limits for virtual memory.
func VmLimits() string {
	return getLimits(syscall.RLIMIT_DATA, "b")
}