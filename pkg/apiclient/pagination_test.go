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

package apiclient

import (
	"net/url"
	"testing"
)

func TestPageOptions_ToQuery_Nil(t *testing.T) {
	var opts *PageOptions
	q := opts.ToQuery(nil)
	if q == nil {
		t.Fatal("expected non-nil url.Values from nil PageOptions")
	}
	if len(q) != 0 {
		t.Fatalf("expected empty query, got %v", q)
	}
}

func TestPageOptions_ToQuery_WithValues(t *testing.T) {
	opts := &PageOptions{Limit: 10, Cursor: "abc"}
	q := opts.ToQuery(nil)
	if q.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", q.Get("limit"))
	}
	if q.Get("cursor") != "abc" {
		t.Fatalf("expected cursor=abc, got %q", q.Get("cursor"))
	}
}

func TestPageOptions_ToQuery_ExistingValues(t *testing.T) {
	opts := &PageOptions{Limit: 5}
	existing := url.Values{"foo": {"bar"}}
	q := opts.ToQuery(existing)
	if q.Get("foo") != "bar" {
		t.Fatal("existing values should be preserved")
	}
	if q.Get("limit") != "5" {
		t.Fatalf("expected limit=5, got %q", q.Get("limit"))
	}
}
