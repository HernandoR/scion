// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hubsync

import (
	"testing"
)

func TestConfirmAction_AutoConfirm(t *testing.T) {
	tests := []struct {
		name       string
		defaultYes bool
	}{
		{"defaultYes=true", true},
		{"defaultYes=false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfirmAction("Test prompt", tt.defaultYes, true)
			if !result {
				t.Errorf("ConfirmAction with autoConfirm=true should always return true, got false (defaultYes=%v)", tt.defaultYes)
			}
		})
	}
}

func TestConfirmAction_NoAutoConfirm_DefaultYes(t *testing.T) {
	// When not auto-confirming and stdin returns EOF/error, it falls back to defaultYes.
	// With defaultYes=true, should return true.
	result := ConfirmAction("Test prompt", true, false)
	if !result {
		t.Error("ConfirmAction with defaultYes=true should return true on stdin EOF")
	}
}

func TestConfirmAction_NoAutoConfirm_DefaultNo(t *testing.T) {
	// When not auto-confirming and stdin returns EOF/error, it falls back to defaultYes.
	// With defaultYes=false, should return false.
	result := ConfirmAction("Test prompt", false, false)
	if result {
		t.Error("ConfirmAction with defaultYes=false should return false on stdin EOF")
	}
}
