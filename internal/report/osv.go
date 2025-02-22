// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/vuln/osv"
	"golang.org/x/vulndb/internal/derrors"
	"golang.org/x/vulndb/internal/stdlib"
)

var (
	// stdFileName is the name of the .json file in the vulndb repo
	// that will contain info on standard library vulnerabilities.
	stdFileName = "stdlib"

	// toolchainFileName is the name of the .json file in the vulndb repo
	// that will contain info on toolchain (cmd/...) vulnerabilities.
	toolchainFileName = "toolchain"

	// osvDir is the name of the directory in the vulndb repo that
	// contains reports.
	OSVDir = "data/osv"

	// schemaVersion is used to indicate which version of the OSV schema a
	// particular vulnerability was exported with.
	schemaVersion = "1.3.1"
)

// GenerateOSVEntry create an osv.Entry for a  In addition to the report, it
// takes the ID for the vuln and a URL that will point to the entry in the vuln DB.
func (r *Report) GenerateOSVEntry(goID string, lastModified time.Time) osv.Entry {
	var credits []osv.Credit
	if r.Credit != "" {
		credits = append(credits, osv.Credit{
			Name: r.Credit,
		})
	}

	entry := osv.Entry{
		ID:            goID,
		Published:     r.Published,
		Modified:      lastModified,
		Withdrawn:     r.Withdrawn,
		Details:       trimWhitespace(r.Description),
		Credits:       credits,
		SchemaVersion: schemaVersion,
	}

	linkName := GetGoAdvisoryLink(goID)
	for _, m := range r.Modules {
		entry.Affected = append(entry.Affected, generateAffected(m, linkName))
	}
	for _, ref := range r.References {
		entry.References = append(entry.References, osv.Reference{
			Type: string(ref.Type),
			URL:  ref.URL,
		})
	}
	entry.Aliases = r.GetAliases()
	return entry
}

func GetOSVFilename(goID string) string {
	return filepath.Join(OSVDir, goID+".json")
}

// ReadOSV reads an osv.Entry from a file.
func ReadOSV(filename string) (entry osv.Entry, err error) {
	derrors.Wrap(&err, "ReadOSV(%s)", filename)
	if err = UnmarshalFromFile(filename, &entry); err != nil {
		return osv.Entry{}, err
	}
	return entry, nil
}

func UnmarshalFromFile(path string, v any) (err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(content, v); err != nil {
		return err
	}
	return nil
}

// ModulesForEntry returns the list of modules affected by an OSV entry.
func ModulesForEntry(entry osv.Entry) []string {
	mods := map[string]bool{}
	for _, a := range entry.Affected {
		mods[a.Package.Name] = true
	}
	return maps.Keys(mods)
}

func AffectedRanges(versions []VersionRange) osv.Affects {
	a := osv.AffectsRange{Type: osv.TypeSemver}
	if len(versions) == 0 || versions[0].Introduced == "" {
		a.Events = append(a.Events, osv.RangeEvent{Introduced: "0"})
	}
	for _, v := range versions {
		if v.Introduced != "" {
			a.Events = append(a.Events, osv.RangeEvent{Introduced: v.Introduced.Canonical()})
		}
		if v.Fixed != "" {
			a.Events = append(a.Events, osv.RangeEvent{Fixed: v.Fixed.Canonical()})
		}
	}
	return osv.Affects{a}
}

// trimWhitespace removes unnecessary whitespace from a string, but preserves
// paragraph breaks (indicated by two newlines).
func trimWhitespace(s string) string {
	s = strings.TrimSpace(s)
	// Replace single newlines with spaces.
	newlines := regexp.MustCompile(`([^\n])\n([^\n])`)
	s = newlines.ReplaceAllString(s, "$1 $2")
	// Replace instances of 2 or more newlines with exactly two newlines.
	paragraphs := regexp.MustCompile(`\s*\n\n\s*`)
	s = paragraphs.ReplaceAllString(s, "\n\n")
	// Replace tabs and double spaces with single spaces.
	spaces := regexp.MustCompile(`[ \t]+`)
	s = spaces.ReplaceAllString(s, " ")
	return s
}

func generateImports(m *Module) (imps []osv.EcosystemSpecificImport) {
	for _, p := range m.Packages {
		syms := append([]string{}, p.Symbols...)
		syms = append(syms, p.DerivedSymbols...)
		sort.Strings(syms)
		imps = append(imps, osv.EcosystemSpecificImport{
			Path:    p.Package,
			GOOS:    p.GOOS,
			GOARCH:  p.GOARCH,
			Symbols: syms,
		})
	}
	return imps
}

func generateAffected(m *Module, url string) osv.Affected {
	name := m.Module
	switch name {
	case stdlib.ModulePath:
		name = stdFileName
	case stdlib.ToolchainModulePath:
		name = toolchainFileName
	}
	return osv.Affected{
		Package: osv.Package{
			Name:      name,
			Ecosystem: osv.GoEcosystem,
		},
		Ranges:           AffectedRanges(m.Versions),
		DatabaseSpecific: osv.DatabaseSpecific{URL: url},
		EcosystemSpecific: osv.EcosystemSpecific{
			Imports: generateImports(m),
		},
	}
}
