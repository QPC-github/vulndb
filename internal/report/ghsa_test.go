// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package report

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/vulndb/internal/ghsa"
)

func TestGHSAToReport(t *testing.T) {
	updatedTime := time.Date(2022, 01, 01, 01, 01, 00, 00, time.UTC)
	sa := &ghsa.SecurityAdvisory{
		ID:          "G1_blah",
		Identifiers: []ghsa.Identifier{{Type: "GHSA", Value: "G1"}, {Type: "CVE", Value: "C1"}},
		UpdatedAt:   updatedTime,
		Permalink:   "https://github.com/permalink/to/G1",
		Description: "a description",
		Vulns: []*ghsa.Vuln{{
			Package:                "aPackage",
			EarliestFixedVersion:   "1.2.3",
			VulnerableVersionRange: "< 1.2.3",
		}},
		References: []ghsa.Reference{{URL: "https://github.com/permalink/to/issue/12345"}},
	}
	for _, test := range []struct {
		name   string
		module string
		want   *Report
	}{
		{
			name:   "module provided",
			module: "aModule",
			want: &Report{
				Modules: []*Module{{
					Module: "aModule",
					Versions: []VersionRange{
						{Fixed: "1.2.3"},
					},
					Packages: []*Package{{
						Package: "aPackage",
					}},
				}},
				Description: "a description",
				GHSAs:       []string{"G1"},
				CVEs:        []string{"C1"},
				References:  []*Reference{{Type: "REPORT", URL: "https://github.com/permalink/to/issue/12345"}},
			},
		},
		{
			name:   "empty module uses package",
			module: "",
			want: &Report{
				Modules: []*Module{{
					Module: "aPackage",
					Versions: []VersionRange{
						{Fixed: "1.2.3"},
					},
					Packages: []*Package{{
						Package: "aPackage",
					}},
				}},
				Description: "a description",
				GHSAs:       []string{"G1"},
				CVEs:        []string{"C1"},
				References:  []*Reference{{Type: "REPORT", URL: "https://github.com/permalink/to/issue/12345"}},
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := GHSAToReport(sa, test.module)
			if diff := cmp.Diff(*got, *test.want); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
func TestParseVulnRange(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []vulnRangeItem
	}{
		{"", nil},
		{"< 1.2.3", []vulnRangeItem{{"<", "1.2.3"}}},
		{"< 4.3.2, >= 1.2.3", []vulnRangeItem{
			{"<", "4.3.2"},
			{">=", "1.2.3"},
		}},
	} {
		got, err := parseVulnRange(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if !cmp.Equal(got, test.want, cmp.AllowUnexported(vulnRangeItem{})) {
			t.Errorf("%q:\ngot  %+v\nwant %+v", test.in, got, test.want)
		}
	}
}

func TestVersions(t *testing.T) {
	for _, test := range []struct {
		earliestFixed string
		vulnRange     string
		intro, fixed  Version
	}{
		{"1.0.0", "< 1.0.0", "", "1.0.0"},
		{"", "<= 1.4.2", "", ""},
		{"1.1.3", ">= 1.1.0, < 1.1.3", "1.1.0", "1.1.3"},
		{
			"1.2.3", "<= 2.3.4",
			`TODO (earliest fixed "1.2.3", vuln range "<= 2.3.4")`, "",
		},
	} {
		got := versions(test.earliestFixed, test.vulnRange)
		want := []VersionRange{{
			Introduced: test.intro,
			Fixed:      test.fixed,
		}}
		if !cmp.Equal(got, want) {
			t.Errorf("%q, %q:\ngot  %+v\nwant %+v",
				test.earliestFixed, test.vulnRange, got, want)
		}
	}
}
