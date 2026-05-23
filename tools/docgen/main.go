/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"html/template"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type config struct {
	root       string
	outDir     string
	modulePath string
	themeStyle string
}

type markdownDoc struct {
	Title    string
	RelPath  string
	Source   string
	HTMLFile string
}

type apiDoc struct {
	ImportPath string
	Title      string
	ShortPath  string
	Section    string
	BodyHTML   template.HTML
	HTMLFile   string
}

type apiPackage struct {
	ImportPath string
	Dir        string
}

type commandHelpRow struct {
	Name        string
	Description string
}

type commandHelpFlag struct {
	Name        string
	Type        string
	Description string
}

type commandHelpDoc struct {
	Title       string
	Command     string
	Synopsis    string
	Summary     []string
	Usage       []string
	Subcommands []commandHelpRow
	Flags       []commandHelpFlag
	ExitCodes   []commandHelpRow
	Raw         string
}

type appManifest struct {
	Name string `json:"name"`
}

type apiSchema struct {
	Service  apiSchemaService  `json:"service"`
	Imports  []string          `json:"imports,omitempty"`
	Types    []apiSchemaType   `json:"types,omitempty"`
	Requests []apiSchemaMethod `json:"requests,omitempty"`
	Events   []apiSchemaMethod `json:"events,omitempty"`
}

type apiSchemaService struct {
	Name        string     `json:"name"`
	ID          flexJSONID `json:"id"`
	Package     string     `json:"package,omitempty"`
	Description string     `json:"description,omitempty"`
}

type apiSchemaType struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Fields      []apiSchemaField `json:"fields,omitempty"`
}

type apiSchemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type apiSchemaMethod struct {
	Name                string     `json:"name"`
	ID                  flexJSONID `json:"id"`
	RequestType         string     `json:"request_type,omitempty"`
	ResponseType        string     `json:"response_type,omitempty"`
	PayloadType         string     `json:"payload_type,omitempty"`
	OneWay              bool       `json:"one_way,omitempty"`
	Description         string     `json:"description,omitempty"`
	RequestDescription  string     `json:"request_description,omitempty"`
	ResponseDescription string     `json:"response_description,omitempty"`
}

type flexJSONID string

func (v *flexJSONID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*v = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*v = flexJSONID(strings.TrimSpace(s))
		return nil
	}
	*v = flexJSONID(strings.TrimSpace(string(data)))
	return nil
}

type docsIndexMeta struct {
	Order  map[string]int
	Titles map[string]string
}

type navEntry struct {
	Title  string
	Href   string
	Active bool
}

type navGroup struct {
	Title   string
	Entries []navEntry
}

type navSection struct {
	Title  string
	Groups []navGroup
}

type pageData struct {
	Head        template.HTML
	CurrentPath string
	Sidebar     []navSection
	BodyHTML    template.HTML
}

type markdownSection struct {
	Title   string
	Content string
}

type markdownCardsTemplateData struct {
	Sections []markdownSection
}

type sectionMarkdownTemplateData struct {
	Title    string
	RelPath  string
	Markdown string
}

type commandReferenceTemplateData struct {
	Title           string
	RelPath         string
	Lead            string
	UsagePrimary    string
	CommandName     string
	HelpSource      string
	DocSource       string
	OverviewHTML    template.HTML
	Usage           []string
	Subcommands     []commandHelpRow
	Flags           []commandHelpFlag
	ExitCodes       []commandHelpRow
	CaptureErr      string
	NoFlagsAndNoDoc bool
	ShowRawHelp     bool
	RawHelp         string
}

type appReferenceTemplateData struct {
	Title        string
	RelPath      string
	Lead         string
	DocSource    string
	PreviewPath  string
	Description  template.HTML
	Flags        []commandHelpFlag
	Subcommands  []commandHelpRow
	FlagsMessage string
	CaptureErr   string
}

type apiSchemaTemplateData struct {
	Title   string
	RelPath string
	Lead    string
	Spec    apiSchema
}

type projectPageTemplateData struct {
	Title     string
	RelPath   string
	CardsHTML template.HTML
}

type projectIndexTemplateData struct {
	Docs []markdownDoc
}

type readmeIndexTemplateData struct {
	RelPath   string
	CardsHTML template.HTML
}

var (
	titleTagPattern       = regexp.MustCompile(`(?is)<title>.*?</title>`)
	descriptionMetaTag    = regexp.MustCompile(`(?is)<meta[^>]*name=["']description["'][^>]*>`)
	scriptTagPattern      = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	stylesheetLinkPattern = regexp.MustCompile(`(?is)<link[^>]*href=["'][^"']*styles\.css[^"']*["'][^>]*>`)

	markdownLinkPattern = regexp.MustCompile(`(!?\[[^\]]*\]\()([^)]+)(\))`)
	htmlAttrLinkPattern = regexp.MustCompile(`(?i)\b(src|href)=["']([^"']+)["']`)

	mdHeadingPattern      = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdUnorderedPattern    = regexp.MustCompile(`^([ \t]*)([-*+])\s+(.+)$`)
	mdOrderedPattern      = regexp.MustCompile(`^([ \t]*)(\d+)\.\s+(.+)$`)
	mdTableDividerPattern = regexp.MustCompile(`^\s*\|?[\s:-]+\|[\s|:-]*$`)
	mdImagePattern        = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	mdLinkPattern         = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdCodePattern         = regexp.MustCompile("`([^`]+)`")
	mdBoldPattern         = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItalicPattern       = regexp.MustCompile(`\*([^*]+)\*`)
	mdStrikePattern       = regexp.MustCompile(`~~([^~]+)~~`)
	htmlOpenTagPattern    = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9:-]*)(\s[^>]*)?>$`)
	htmlCloseTagPattern   = regexp.MustCompile(`^</([A-Za-z][A-Za-z0-9:-]*)\s*>$`)
	htmlSelfTagPattern    = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9:-]*)(\s[^>]*)?/\s*>$`)
	commandFlagLineRegex  = regexp.MustCompile(`^\s*-(\S+)(?:\s+(.+))?$`)
)

//go:embed templates/*.tmpl
var docTemplateFS embed.FS

var docTemplates = template.Must(template.New("docgen").Funcs(template.FuncMap{
	"markdown":            renderMarkdown,
	"trim":                strings.TrimSpace,
	"join":                strings.Join,
	"defaultText":         defaultText,
	"methodPayloadType":   methodPayloadType,
	"responseType":        responseType,
	"requestDescription":  requestDescription,
	"responseDescription": responseDescription,
	"payloadDescription":  payloadDescription,
}).ParseFS(docTemplateFS, "templates/*.tmpl"))

const fallbackHead = `<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="color-scheme" content="light dark" />
  <title>avyos — Dharmic Documentation</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link rel="stylesheet" href="styles.css" />
</head>`

const docgenStyles = `
/* Dharmic design tokens for documentation components */
:root {
  --doc-link-hover: rgba(201, 75, 10, 0.09);
  --doc-link-active-bg: rgba(201, 75, 10, 0.14);
  --doc-link-active-border: rgba(201, 75, 10, 0.5);
  --doc-blockquote-border: rgba(201, 75, 10, 0.5);
  --doc-blockquote-bg: rgba(201, 75, 10, 0.07);
  --doc-code-bg: rgba(28, 13, 0, 0.06);
  --doc-code-shadow: inset 0 0 0 1px rgba(201, 148, 48, 0.18);
  --doc-code-top: rgba(201, 148, 48, 0.6);
  --doc-inline-code-bg: rgba(201, 75, 10, 0.08);
  --doc-table-head-bg: rgba(201, 75, 10, 0.08);
  --doc-card-bg: rgba(254, 248, 234, 0.92);
  --doc-api-card-bg: rgba(254, 246, 230, 0.94);
  --doc-io-card-bg: rgba(255, 250, 240, 0.97);
  --doc-subitem-bg: rgba(253, 247, 232, 0.96);
  --doc-card-shadow: 0 4px 14px rgba(92, 48, 12, 0.09);
  --doc-api-card-shadow: 0 4px 12px rgba(92, 48, 12, 0.07);
  --doc-callout-border: rgba(201, 75, 10, 0.52);
  --doc-callout-bg: rgba(201, 75, 10, 0.07);
  --doc-scrollbar-color: rgba(201, 75, 10, 0.4);
  --doc-scrollbar-thumb: linear-gradient(180deg, rgba(201, 75, 10, 0.5), rgba(201, 148, 48, 0.44));
  --doc-scrollbar-thumb-hover: linear-gradient(180deg, rgba(201, 75, 10, 0.68), rgba(201, 148, 48, 0.6));
  --doc-header-bg: rgba(253, 244, 227, 0.88);
  --doc-header-border: rgba(201, 148, 48, 0.2);
  --doc-backdrop: rgba(28, 13, 0, 0.48);
  --doc-sidebar-bg: rgba(254, 249, 238, 0.98);
  --doc-sidebar-border: rgba(201, 148, 48, 0.3);
  --doc-sidebar-pillar: rgba(201, 148, 48, 0.36);
  --doc-nav-toggle-hover: rgba(201, 75, 10, 0.07);
  --doc-nav-dot: #C94B0A;
}

:root[data-theme="dark"] {
  --doc-link-hover: rgba(255, 125, 34, 0.1);
  --doc-link-active-bg: rgba(255, 125, 34, 0.16);
  --doc-link-active-border: rgba(255, 125, 34, 0.45);
  --doc-blockquote-border: rgba(245, 200, 66, 0.45);
  --doc-blockquote-bg: rgba(245, 200, 66, 0.08);
  --doc-code-bg: rgba(8, 4, 16, 0.72);
  --doc-code-shadow: inset 0 0 0 1px rgba(201, 148, 48, 0.22);
  --doc-code-top: rgba(201, 148, 48, 0.5);
  --doc-inline-code-bg: rgba(255, 125, 34, 0.12);
  --doc-table-head-bg: rgba(255, 125, 34, 0.12);
  --doc-card-bg: rgba(28, 18, 40, 0.92);
  --doc-api-card-bg: rgba(22, 14, 32, 0.94);
  --doc-io-card-bg: rgba(18, 10, 28, 0.97);
  --doc-subitem-bg: rgba(30, 20, 44, 0.96);
  --doc-card-shadow: 0 4px 14px rgba(0, 0, 0, 0.3);
  --doc-api-card-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  --doc-callout-border: rgba(245, 200, 66, 0.45);
  --doc-callout-bg: rgba(245, 200, 66, 0.08);
  --doc-scrollbar-color: rgba(255, 125, 34, 0.5);
  --doc-scrollbar-thumb: linear-gradient(180deg, rgba(255, 125, 34, 0.55), rgba(245, 200, 66, 0.4));
  --doc-scrollbar-thumb-hover: linear-gradient(180deg, rgba(255, 125, 34, 0.72), rgba(245, 200, 66, 0.56));
  --doc-header-bg: rgba(12, 8, 18, 0.88);
  --doc-header-border: rgba(201, 148, 48, 0.16);
  --doc-backdrop: rgba(0, 0, 0, 0.62);
  --doc-sidebar-bg: rgba(16, 10, 26, 0.98);
  --doc-sidebar-border: rgba(201, 148, 48, 0.2);
  --doc-sidebar-pillar: rgba(201, 148, 48, 0.24);
  --doc-nav-toggle-hover: rgba(255, 125, 34, 0.08);
  --doc-nav-dot: #FF7D22;
}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme]) {
    --doc-link-hover: rgba(255, 125, 34, 0.1);
    --doc-link-active-bg: rgba(255, 125, 34, 0.16);
    --doc-link-active-border: rgba(255, 125, 34, 0.45);
    --doc-blockquote-border: rgba(245, 200, 66, 0.45);
    --doc-blockquote-bg: rgba(245, 200, 66, 0.08);
    --doc-code-bg: rgba(8, 4, 16, 0.72);
    --doc-code-shadow: inset 0 0 0 1px rgba(201, 148, 48, 0.22);
    --doc-code-top: rgba(201, 148, 48, 0.5);
    --doc-inline-code-bg: rgba(255, 125, 34, 0.12);
    --doc-table-head-bg: rgba(255, 125, 34, 0.12);
    --doc-card-bg: rgba(28, 18, 40, 0.92);
    --doc-api-card-bg: rgba(22, 14, 32, 0.94);
    --doc-io-card-bg: rgba(18, 10, 28, 0.97);
    --doc-subitem-bg: rgba(30, 20, 44, 0.96);
    --doc-card-shadow: 0 4px 14px rgba(0, 0, 0, 0.3);
    --doc-api-card-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
    --doc-callout-border: rgba(245, 200, 66, 0.45);
    --doc-callout-bg: rgba(245, 200, 66, 0.08);
    --doc-scrollbar-color: rgba(255, 125, 34, 0.5);
    --doc-scrollbar-thumb: linear-gradient(180deg, rgba(255, 125, 34, 0.55), rgba(245, 200, 66, 0.4));
    --doc-scrollbar-thumb-hover: linear-gradient(180deg, rgba(255, 125, 34, 0.72), rgba(245, 200, 66, 0.56));
    --doc-header-bg: rgba(12, 8, 18, 0.88);
    --doc-header-border: rgba(201, 148, 48, 0.16);
    --doc-backdrop: rgba(0, 0, 0, 0.62);
    --doc-sidebar-bg: rgba(16, 10, 26, 0.98);
    --doc-sidebar-border: rgba(201, 148, 48, 0.2);
    --doc-sidebar-pillar: rgba(201, 148, 48, 0.24);
    --doc-nav-toggle-hover: rgba(255, 125, 34, 0.08);
    --doc-nav-dot: #FF7D22;
  }
}

/* Brand mark */
.brand-om {
  font-size: 1.48rem;
  line-height: 1;
  font-family: "Noto Serif", serif;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  filter: drop-shadow(0 1px 2px rgba(201, 75, 10, 0.28));
}

.brand-name {
  font-family: var(--display);
  font-weight: 700;
  letter-spacing: -0.02em;
}

.brand-sep {
  color: var(--muted);
  font-weight: 300;
  opacity: 0.6;
}

.brand-sub {
  font-family: var(--display);
  font-weight: 400;
  font-size: 0.88em;
  color: var(--muted);
  letter-spacing: 0.01em;
}

.site-header {
  top: 3px; /* clears the tilak bar */
  border-bottom-color: var(--doc-header-border);
  background: var(--doc-header-bg);
}

.doc-theme-toggle,
.doc-menu-toggle,
.doc-sidebar-close {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--glass);
  color: var(--text);
  font: inherit;
  font-size: 0.84rem;
  font-weight: 700;
  line-height: 1;
  cursor: pointer;
  transition: background-color 0.14s ease, border-color 0.14s ease, transform 0.14s ease;
}

.doc-theme-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  padding: 0.64rem 0.78rem;
  white-space: nowrap;
}

.doc-theme-toggle:hover,
.doc-menu-toggle:hover,
.doc-sidebar-close:hover {
  border-color: var(--border);
  background: var(--doc-link-hover);
}

.doc-theme-toggle:active,
.doc-menu-toggle:active,
.doc-sidebar-close:active {
  transform: translateY(1px);
}

.doc-theme-icon {
  width: 0.72rem;
  height: 0.72rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.42);
}

.doc-theme-label {
  font-family: var(--mono);
  font-size: 0.73rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.doc-menu-toggle {
  display: none;
  padding: 0.64rem 0.78rem;
}

.doc-shell .container {
  width: calc(100% - 44px);
  max-width: none;
  margin-inline: auto;
}

.doc-shell {
  padding-block: clamp(1.35rem, 2.2vw, 2.1rem);
}

.doc-layout {
  position: relative;
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: clamp(0.82rem, 1.5vw, 1.15rem);
  align-items: start;
}

.doc-sidebar-backdrop {
  position: fixed;
  inset: 0;
  border: 0;
  margin: 0;
  padding: 0;
  background: var(--doc-backdrop);
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.18s ease;
  z-index: 72;
}

.doc-sidebar-backdrop[hidden] {
  display: none;
}

.doc-sidebar-backdrop.open {
  opacity: 1;
  pointer-events: auto;
}

.doc-sidebar {
  position: sticky;
  top: 92px;
  max-height: calc(100vh - 112px);
  overflow: auto;
  padding: 0;
  background: var(--doc-sidebar-bg);
  border: 1px solid var(--doc-sidebar-border);
  border-radius: 12px;
  border-left: 3px solid var(--doc-sidebar-pillar);
  box-shadow: 0 2px 12px rgba(92, 48, 12, 0.08);
  scrollbar-gutter: stable;
  scrollbar-width: thin;
  scrollbar-color: var(--doc-scrollbar-color) transparent;
}

.doc-sidebar::-webkit-scrollbar {
  width: 10px;
}

.doc-sidebar::-webkit-scrollbar-track {
  background: transparent;
  margin: 4px 0;
  border-radius: 999px;
}

.doc-sidebar::-webkit-scrollbar-thumb {
  border-radius: 999px;
  border: 3px solid transparent;
  background-clip: padding-box;
  background: var(--doc-scrollbar-thumb);
  min-height: 34px;
}

.doc-sidebar:hover::-webkit-scrollbar-thumb {
  background: var(--doc-scrollbar-thumb-hover);
}

.doc-sidebar-head {
  display: none;
  align-items: center;
  gap: 0.5rem;
  padding: 0.72rem 0.9rem 0.68rem;
  border-bottom: 1px solid var(--doc-sidebar-border);
}

.doc-sidebar-om {
  font-size: 1.22rem;
  font-family: "Noto Serif", serif;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  line-height: 1;
}

.doc-sidebar-title {
  flex: 1;
  margin: 0;
  font-family: var(--display);
  font-size: 0.8rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
}

.doc-sidebar-close {
  padding: 0.34rem 0.52rem;
  margin-left: auto;
}

/* Section toggle */
.doc-nav-group + .doc-nav-group {
  border-top: 1px solid var(--doc-sidebar-border);
}

.doc-nav-section-toggle {
  display: flex;
  align-items: center;
  gap: 0.52rem;
  width: 100%;
  padding: 0.62rem 0.9rem;
  background: none;
  border: 0;
  cursor: pointer;
  color: var(--text);
  font: inherit;
  font-family: var(--display);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  text-align: left;
  transition: background 0.14s ease, color 0.14s ease;
}

.doc-nav-section-toggle:hover {
  background: var(--doc-nav-toggle-hover);
}

.doc-nav-section-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--doc-nav-dot);
  opacity: 0.7;
  transition: opacity 0.14s ease, transform 0.14s ease;
}

.doc-nav-section-toggle:hover .doc-nav-section-dot,
.doc-nav-section-toggle[aria-expanded="true"] .doc-nav-section-dot {
  opacity: 1;
  transform: scale(1.2);
}

.doc-nav-section-title {
  flex: 1;
  min-width: 0;
}

.doc-nav-section-chevron {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  color: var(--muted);
  transition: transform 0.18s ease;
}

.doc-nav-section-toggle[aria-expanded="false"] .doc-nav-section-chevron {
  transform: rotate(-90deg);
}

.doc-nav-section-body {
  padding: 0 0.56rem 0.56rem;
}

.doc-nav-section-body[hidden] {
  display: none;
}

.doc-subtitle {
  margin: 0.32rem 0 0.1rem 0.22rem;
  padding: 0 0.36rem;
  font-family: var(--mono);
  font-size: 0.67rem;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: var(--muted);
}

.doc-link {
  display: block;
  text-decoration: none;
  color: var(--muted);
  padding: 0.34rem 0.52rem 0.34rem 0.62rem;
  border-left: 2px solid transparent;
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  transition: background-color 0.13s ease, border-color 0.13s ease, color 0.13s ease;
  font-size: 0.88rem;
  line-height: 1.32;
}

.doc-link:hover {
  color: var(--text);
  border-left-color: var(--doc-sidebar-pillar);
  background: var(--doc-link-hover);
}

.doc-link.active {
  color: var(--accent);
  font-weight: 700;
  border-left-color: var(--accent);
  background: var(--doc-link-active-bg);
}

.doc-content {
  padding: 0;
  min-height: 80vh;
  min-width: 0;
  background: transparent;
  border: 0;
  box-shadow: none;
}

.doc-content.card {
  background: transparent;
  border: 0;
  box-shadow: none;
}

.doc-content > *:first-child {
  margin-top: 0;
}

.doc-content > * {
  max-width: 1120px;
  margin-inline: auto;
}

.doc-path {
  margin-top: 0;
  margin-bottom: 0.48rem;
  letter-spacing: 0.01em;
  font-size: 0.86rem;
}

.doc-list {
  margin: 0.22rem 0 0;
  padding-left: 1.2rem;
}

.doc-list li + li {
  margin-top: 0.24rem;
}

.doc-list a {
  color: var(--accent);
}

.doc-markdown,
.doc-api {
  max-width: none;
}

.doc-markdown h1,
.doc-markdown h2,
.doc-markdown h3,
.doc-markdown h4,
.doc-api h1,
.doc-api h2,
.doc-api h3,
.doc-api h4 {
  font-family: var(--display);
  letter-spacing: -0.02em;
  line-height: 1.14;
  margin-top: 1.05rem;
  margin-bottom: 0.46rem;
}

.doc-markdown h1,
.doc-api h1 {
  margin-top: 0;
  font-size: clamp(1.58rem, 2.1vw, 2.05rem);
}

.doc-markdown h2,
.doc-api h2 {
  font-size: clamp(1.14rem, 1.6vw, 1.42rem);
  padding-top: 0.18rem;
  border-top: 1px solid var(--border);
}

.doc-markdown p,
.doc-api p {
  margin: 0.54rem 0;
  color: var(--text);
  line-height: 1.56;
  overflow-wrap: anywhere;
  word-break: normal;
}

.doc-markdown ul,
.doc-markdown ol,
.doc-api ul,
.doc-api ol {
  margin: 0.52rem 0;
  padding-left: 1.2rem;
}

.doc-markdown li + li,
.doc-api li + li {
  margin-top: 0.2rem;
}

.doc-markdown li,
.doc-api li,
.doc-markdown td,
.doc-markdown th,
.doc-api td,
.doc-api th {
  overflow-wrap: anywhere;
  word-break: normal;
}

.doc-markdown blockquote,
.doc-api blockquote {
  margin: 0.68rem 0;
  padding: 0.56rem 0.72rem;
  border-left: 4px solid var(--doc-blockquote-border);
  background: var(--doc-blockquote-bg);
  border-radius: var(--radius-sm);
}

.doc-markdown pre,
.doc-api pre {
  overflow: auto;
  margin: 0.72rem 0;
  padding: 0.72rem 0.86rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  border-top: 3px solid var(--doc-code-top);
  background: var(--doc-code-bg);
  box-shadow: var(--doc-code-shadow);
}

.doc-markdown pre code,
.doc-api pre code {
  font-size: 0.86rem;
  line-height: 1.45;
}

.doc-markdown code,
.doc-api code {
  font-family: var(--mono);
  font-size: 0.9rem;
}

.doc-markdown p code,
.doc-markdown li code,
.doc-api p code,
.doc-api li code {
  padding: 0.08rem 0.24rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--doc-inline-code-bg);
}

.doc-markdown table,
.doc-ref-table {
  width: 100%;
  margin: 0.65rem 0 0.95rem;
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.doc-markdown th,
.doc-markdown td,
.doc-ref-table th,
.doc-ref-table td {
  border-bottom: 1px solid var(--border);
  border-right: 1px solid var(--border);
  text-align: left;
  padding: 0.44rem 0.54rem;
  vertical-align: top;
}

.doc-markdown th:last-child,
.doc-markdown td:last-child,
.doc-ref-table th:last-child,
.doc-ref-table td:last-child {
  border-right: 0;
}

.doc-markdown tr:last-child td,
.doc-ref-table tr:last-child td {
  border-bottom: 0;
}

.doc-markdown th,
.doc-ref-table th {
  background: var(--doc-table-head-bg);
  font-family: var(--display);
  font-size: 0.88rem;
}

.doc-ref-table th {
  font-size: 0.84rem;
}

.doc-markdown a {
  color: var(--accent);
  text-decoration-thickness: 1px;
  text-underline-offset: 2px;
}

.doc-markdown hr,
.doc-api hr {
  border: 0;
  border-top: 1px solid var(--border);
  margin: 0.9rem 0;
}

.doc-markdown img,
.doc-api img {
  max-width: 100%;
  height: auto;
  display: block;
  border-radius: 10px;
  border: 1px solid var(--border);
  box-shadow: var(--shadow-2);
  margin: 0.62rem 0;
}

.doc-ref-shell {
  display: grid;
  gap: 0.75rem;
}

.doc-ref-hero {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 0.74rem 0.84rem;
  background: var(--doc-card-bg);
  box-shadow: var(--doc-card-shadow);
}

.doc-ref-hero h1 {
  margin: 0;
  font-family: var(--display);
  font-size: clamp(1.32rem, 1.9vw, 1.7rem);
  line-height: 1.1;
  background: linear-gradient(135deg, var(--text) 60%, var(--accent));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.doc-ref-hero .doc-path {
  margin-top: 0.2rem;
  margin-bottom: 0;
}

.doc-ref-lead {
  margin: 0.38rem 0 0;
  color: var(--text);
  line-height: 1.4;
}

.doc-ref-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 0.72rem;
}

.doc-ref-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--doc-card-bg);
  padding: 0.68rem 0.78rem;
  box-shadow: var(--doc-card-shadow);
}

.doc-ref-card h2 {
  margin: 0 0 0.45rem;
  font-size: 1rem;
  font-family: var(--display);
}

.doc-ref-meta {
  margin: 0;
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.34rem 0.56rem;
}

.doc-ref-meta dt {
  margin: 0;
  font-size: 0.78rem;
  font-family: var(--mono);
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.doc-ref-meta dd {
  margin: 0;
  color: var(--text);
}

.doc-ref-preview {
  margin: 0;
}

.doc-ref-preview img {
  width: 100%;
  max-width: 560px;
  height: auto;
  border-radius: 10px;
  border: 1px solid var(--border);
  box-shadow: var(--shadow-2);
}

.doc-ref-preview figcaption {
  margin-top: 0.32rem;
  color: var(--muted);
  font-size: 0.8rem;
}

.doc-ref-empty {
  margin: 0;
  color: var(--muted);
  font-style: italic;
}

.doc-ref-list {
  margin: 0;
  padding-left: 1.05rem;
}

.doc-ref-list li + li {
  margin-top: 0.25rem;
}

.doc-ref-code {
  margin: 0;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 9px;
  padding: 0.56rem 0.66rem;
  background: var(--doc-code-bg);
}

.doc-ref-callout {
  margin: 0;
  border: 1px solid var(--border);
  border-left: 4px solid var(--doc-callout-border);
  border-radius: 9px;
  padding: 0.52rem 0.62rem;
  background: var(--doc-callout-bg);
  color: var(--text);
}

.doc-card-stack {
  display: grid;
  gap: 0.72rem;
}

.doc-section-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--doc-card-bg);
  padding: 0.74rem 0.84rem;
  box-shadow: var(--doc-card-shadow);
}

.doc-section-card > h2 {
  margin: 0 0 0.42rem;
  font-family: var(--display);
  font-size: 1.06rem;
}

.api-card-list {
  display: grid;
  gap: 0.68rem;
}

.api-item-card {
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--doc-api-card-bg);
  padding: 0.62rem 0.68rem;
  box-shadow: var(--doc-api-card-shadow);
}

.api-item-card > h3,
.api-item-head h3 {
  margin: 0;
  font-family: var(--display);
  font-size: 0.96rem;
}

.api-item-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.45rem;
}

.api-item-id {
  margin: 0;
}

.api-io-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.58rem;
  margin-top: 0.56rem;
}

.api-io-card {
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--doc-io-card-bg);
  padding: 0.52rem 0.58rem;
}

.api-io-card h4 {
  margin: 0 0 0.24rem;
  font-family: var(--display);
  font-size: 0.9rem;
}

.api-io-card p {
  margin: 0.28rem 0;
}

.doc-go-shell {
  display: grid;
  gap: 0.76rem;
}

.doc-go-group > h2 {
  margin: 0 0 0.5rem;
  border-top: 0;
  padding-top: 0;
}

.doc-go-symbol {
  margin: 0;
  color: var(--muted);
  font-size: 0.82rem;
}

.doc-go-item pre,
.doc-go-subitem pre {
  margin: 0.4rem 0 0;
}

.doc-go-item .doc-markdown,
.doc-go-subitem .doc-markdown {
  margin-top: 0.44rem;
}

.doc-go-subgroup {
  margin-top: 0.56rem;
  padding-top: 0.48rem;
  border-top: 1px solid var(--border);
}

.doc-go-subgroup > h4 {
  margin: 0 0 0.4rem;
  font-family: var(--display);
  font-size: 0.9rem;
}

.doc-go-sublist {
  gap: 0.52rem;
}

.doc-go-subitem {
  padding: 0.52rem 0.58rem;
  background: var(--doc-subitem-bg);
}

@media (max-width: 1080px) {
  .doc-shell .container {
    width: calc(100% - 24px);
  }

  .doc-sidebar {
    top: 82px;
    max-height: calc(100vh - 100px);
  }

  .doc-theme-label {
    display: none;
  }

  .doc-theme-toggle {
    padding: 0.64rem;
  }
}

@media (max-width: 940px) {
  body.doc-nav-open {
    overflow: hidden;
  }

  .doc-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .doc-menu-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .doc-shell .container {
    width: calc(100% - 16px);
  }

  .doc-sidebar {
    position: fixed;
    top: 84px;
    left: 8px;
    width: min(86vw, 340px);
    max-height: calc(100vh - 98px);
    z-index: 80;
    overflow: auto;
    transform: translateX(calc(-100% - 20px));
    transition: transform 0.22s cubic-bezier(0.32, 0.72, 0, 1);
    box-shadow: 0 8px 32px rgba(28, 13, 0, 0.24);
  }

  .doc-sidebar.open {
    transform: translateX(0);
  }

  .doc-content {
    min-height: 0;
  }

  .doc-content > * {
    max-width: 100%;
  }
}

@media (max-width: 700px) {
  .nav-actions {
    gap: 0.4rem;
  }

  .doc-theme-toggle {
    padding: 0.56rem 0.62rem;
    font-size: 0.78rem;
  }

  .doc-menu-toggle {
    padding: 0.56rem 0.62rem;
    font-size: 0.78rem;
  }

  .doc-ref-hero,
  .doc-ref-card,
  .doc-section-card,
  .api-item-card,
  .api-io-card {
    padding-inline: 0.68rem;
  }

  .doc-markdown table,
  .doc-ref-table {
    display: block;
    overflow-x: auto;
    white-space: nowrap;
  }

  .doc-markdown th,
  .doc-markdown td,
  .doc-ref-table th,
  .doc-ref-table td {
    min-width: 128px;
  }
}

/* Heading anchors */
.doc-anchor {
  display: inline-block;
  margin-right: 0.36em;
  color: var(--muted);
  text-decoration: none;
  font-weight: 400;
  opacity: 0;
  transition: opacity 0.14s ease;
  font-size: 0.85em;
}

h1:hover .doc-anchor,
h2:hover .doc-anchor,
h3:hover .doc-anchor,
h4:hover .doc-anchor,
h5:hover .doc-anchor,
h6:hover .doc-anchor {
  opacity: 1;
}

/* Code block copy button */
.doc-code-wrap {
  position: relative;
}

.doc-copy-btn {
  position: absolute;
  top: 0.46rem;
  right: 0.46rem;
  padding: 0.28rem 0.56rem;
  font-family: var(--mono);
  font-size: 0.72rem;
  line-height: 1;
  font-weight: 600;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--glass);
  color: var(--muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.14s ease, background 0.14s ease, color 0.14s ease;
}

.doc-code-wrap:hover .doc-copy-btn {
  opacity: 1;
}

.doc-copy-btn:hover {
  background: var(--doc-link-hover);
  color: var(--text);
}

.doc-copy-btn.copied {
  color: #22863a;
  border-color: rgba(34, 134, 58, 0.4);
}

/* Sidebar search */
.doc-search-wrap {
  position: relative;
  padding: 0.52rem 0.72rem 0.56rem;
  border-bottom: 1px solid var(--doc-sidebar-border);
}

.doc-search-icon {
  position: absolute;
  left: 1.22rem;
  top: 50%;
  transform: translateY(-50%);
  width: 14px;
  height: 14px;
  color: var(--muted);
  pointer-events: none;
}

.doc-search {
  width: 100%;
  box-sizing: border-box;
  padding: 0.44rem 0.62rem 0.44rem 2rem;
  font-family: var(--mono);
  font-size: 0.82rem;
  border: 1px solid var(--doc-sidebar-border);
  border-radius: 999px;
  background: var(--glass);
  color: var(--text);
  outline: none;
  transition: border-color 0.14s ease, box-shadow 0.14s ease;
}

.doc-search:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(201, 75, 10, 0.16);
}

.doc-search::placeholder {
  color: var(--muted);
}

/* "On this page" TOC */
.doc-toc {
  position: sticky;
  top: 92px;
  max-height: calc(100vh - 112px);
  overflow: auto;
  width: 220px;
  flex-shrink: 0;
  padding: 0.72rem 0.66rem;
  font-size: 0.82rem;
  align-self: start;
  display: none;
}

.doc-toc-title {
  margin: 0 0 0.46rem;
  padding: 0 0.18rem;
  font-family: var(--display);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
}

.doc-toc-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.doc-toc-list li {
  margin: 0;
}

.doc-toc-list a {
  display: block;
  padding: 0.26rem 0.52rem;
  color: var(--muted);
  text-decoration: none;
  border-left: 2px solid transparent;
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  transition: color 0.12s ease, border-color 0.12s ease, background 0.12s ease;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.doc-toc-list a:hover {
  color: var(--text);
  background: var(--doc-link-hover);
}

.doc-toc-list a.active {
  color: var(--accent);
  border-left-color: var(--accent);
  background: var(--doc-link-active-bg);
}

.doc-toc-list .toc-h3 a {
  padding-left: 1.1rem;
  font-size: 0.78rem;
}

@media (min-width: 1380px) {
  .doc-layout {
    grid-template-columns: minmax(260px, 300px) minmax(0, 1fr) 220px;
  }

  .doc-toc {
    display: block;
  }
}
`

func main() {
	var cfg config
	flag.StringVar(&cfg.outDir, "out", "_cache/docs", "Output directory for generated docs")
	flag.StringVar(&cfg.modulePath, "module", "", "Go module path (auto-detected from go.mod if empty)")
	flag.StringVar(&cfg.themeStyle, "theme-style", "docs/styles.css", "Theme source styles.css")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
	cfg.root = root

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.modulePath == "" {
		path, err := detectModulePath(filepath.Join(cfg.root, "go.mod"))
		if err != nil {
			return err
		}
		cfg.modulePath = path
	}

	outDir := resolvePath(cfg.root, cfg.outDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(outDir, "project-index.html"))
	_ = os.Remove(filepath.Join(outDir, "project-readme.html"))

	projectDocs, err := collectProjectDocs(cfg.root)
	if err != nil {
		return err
	}
	indexMeta := parseDocsIndex(projectDocs)

	homeRelPath := ""
	if _, ok := findProjectDoc(projectDocs, "README.md"); ok {
		homeRelPath = "README.md"
	} else if _, ok := findProjectDoc(projectDocs, "docs/index.md"); ok {
		homeRelPath = "docs/index.md"
	}

	markdownMap := map[string]string{}
	for i := range projectDocs {
		if projectDocs[i].RelPath == homeRelPath {
			markdownMap[projectDocs[i].RelPath] = "index.html"
		} else {
			markdownMap[projectDocs[i].RelPath] = projectDocs[i].HTMLFile
		}
	}

	appDocs, err := renderAppDocs(cfg.root)
	if err != nil {
		return err
	}
	commandDocs, err := renderCommandDocs(cfg.root)
	if err != nil {
		return err
	}
	serviceDocs, err := renderProgramDocs(cfg.root, "services", []string{"docs.go", "doc.go"})
	if err != nil {
		return err
	}
	apiDocs, err := renderAPIJSONDocs(cfg.root)
	if err != nil {
		return err
	}
	pkgDocs, err := renderPkgDocs(cfg.root, cfg.modulePath)
	if err != nil {
		return err
	}
	allGeneratedDocs := make([]apiDoc, 0, len(appDocs)+len(commandDocs)+len(serviceDocs)+len(apiDocs)+len(pkgDocs))
	allGeneratedDocs = append(allGeneratedDocs, appDocs...)
	allGeneratedDocs = append(allGeneratedDocs, commandDocs...)
	allGeneratedDocs = append(allGeneratedDocs, serviceDocs...)
	allGeneratedDocs = append(allGeneratedDocs, apiDocs...)
	allGeneratedDocs = append(allGeneratedDocs, pkgDocs...)

	if err := copyFile(resolvePath(cfg.root, cfg.themeStyle), filepath.Join(outDir, "styles.css")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "docgen.css"), []byte(docgenStyles), 0644); err != nil {
		return err
	}
	if err := copyDirIfExists(filepath.Join(cfg.root, "docs", "assets"), filepath.Join(outDir, "assets")); err != nil {
		return err
	}
	if err := copyAppPreviewAssets(cfg.root, outDir); err != nil {
		return err
	}
	if err := copyOptionalFile(filepath.Join(cfg.root, "data", "icons", "logo", "logo.png"), filepath.Join(outDir, "assets", "logo.png")); err != nil {
		return err
	}

	for _, d := range projectDocs {
		if d.RelPath == homeRelPath {
			continue
		}
		rewritten := rewriteDocLinks(d.Source, d.RelPath, markdownMap)
		body := renderProjectBody(d.Title, d.RelPath, rewritten)
		head := buildHead(d.Title+" | avyos Docs", "Project documentation for "+d.Title)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, d.HTMLFile),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	for _, d := range allGeneratedDocs {
		body := renderAPIBody(d)
		head := buildHead(d.ImportPath+" | avyos Docs", "Documentation for "+d.ImportPath)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, d.HTMLFile),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	var homeBody template.HTML
	homeTitle := "Project Documentation | avyos Docs"
	homeDesc := "Project documentation and API reference for avyos"
	if homeRelPath != "" {
		homeDoc, _ := findProjectDoc(projectDocs, homeRelPath)
		rewritten := rewriteDocLinks(homeDoc.Source, homeDoc.RelPath, markdownMap)
		homeBody = renderReadmeIndexBody(homeDoc.RelPath, rewritten)
		homeTitle = homeDoc.Title + " | avyos Docs"
		homeDesc = "Documentation index for avyos"
	} else {
		homeBody = renderProjectIndexBody(projectDocs)
	}
	home := pageData{
		Head:        buildHead(homeTitle, homeDesc),
		CurrentPath: "index.html",
		Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, "index.html"),
		BodyHTML:    homeBody,
	}
	if err := renderPage(filepath.Join(outDir, "index.html"), home); err != nil {
		return err
	}

	fmt.Printf("[*] Project docs: %d\n", len(projectDocs))
	fmt.Printf("[*] Generated docs: %d\n", len(allGeneratedDocs))
	fmt.Printf("[✓] Documentation site generated at %s\n", filepath.Join(outDir, "index.html"))
	return nil
}

func collectProjectDocs(root string) ([]markdownDoc, error) {
	var files []string

	readme := filepath.Join(root, "README.md")
	if st, err := os.Stat(readme); err == nil && !st.IsDir() {
		files = append(files, readme)
	}

	docsDir := filepath.Join(root, "docs")
	if st, err := os.Stat(docsDir); err == nil && st.IsDir() {
		if err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || filepath.Base(path) == "index.md" {
				return nil
			}
			if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	if len(files) == 0 {
		return nil, errors.New("no markdown files found (expected README.md and/or docs/*.md)")
	}

	usedNames := map[string]int{}
	docs := make([]markdownDoc, 0, len(files))
	for _, p := range files {
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		title := markdownTitle(string(content), strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)))
		baseName := "project-" + slugify(strings.TrimSuffix(rel, filepath.Ext(rel)))
		htmlName := uniqueHTMLName(baseName, usedNames)
		docs = append(docs, markdownDoc{
			Title:    title,
			RelPath:  rel,
			Source:   string(content),
			HTMLFile: htmlName,
		})
	}

	return docs, nil
}

func collectGoPackages(root, modulePath, prefix string, includeMain bool) ([]apiPackage, error) {
	var packages []apiPackage
	searchRoot := filepath.Join(root, prefix)
	st, err := os.Stat(searchRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		pkgName, hasGoFiles, err := packageNameForDir(path)
		if err != nil {
			return err
		}
		if !hasGoFiles {
			return nil
		}
		if includeMain {
			if pkgName != "main" {
				return nil
			}
		} else if pkgName == "main" {
			return nil
		}

		importPath := modulePath
		if rel != "." {
			importPath += "/" + rel
		}
		packages = append(packages, apiPackage{
			ImportPath: importPath,
			Dir:        path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})
	return packages, nil
}

func collectMainProgramDirs(root, prefix string) ([]string, error) {
	searchRoot := filepath.Join(root, prefix)
	st, err := os.Stat(searchRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var dirs []string
	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		mainFile := filepath.Join(path, "main.go")
		if _, statErr := os.Stat(mainFile); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil
			}
			return statErr
		}

		pkgName, hasGoFiles, pkgErr := packageNameForDir(path)
		if pkgErr != nil {
			return pkgErr
		}
		if !hasGoFiles || pkgName != "main" {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			return nil
		}
		seen[rel] = struct{}{}
		dirs = append(dirs, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)
	return dirs, nil
}

func renderProgramDocs(root, section string, preferredDocFiles []string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, section)
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		docText, sourceFile, readErr := readProgramDoc(root, relDir, preferredDocFiles)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    section,
			BodyHTML:   renderProgramReferenceBody(section, title, relDir, docText, sourceFile),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func renderAppDocs(root string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, "apps")
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		manifestName, manifestErr := appDisplayName(root, relDir)
		if manifestErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read app manifest for %s: %v\n", relDir, manifestErr)
		} else if manifestName != "" {
			title = manifestName
		}

		docText, sourceFile, readErr := readProgramDoc(root, relDir, []string{"docs.go", "doc.go"})
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		previewPath := appPreviewPath(root, relDir)
		usage := ""
		captureErr := ""
		usesFlags, detectErr := commandUsesFlagPackage(filepath.Join(root, relDir))
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to inspect app flags for %s: %v\n", relDir, detectErr)
			usesFlags = true
		}
		if usesFlags {
			captured, usageErr := commandUsageText(root, relDir)
			if usageErr != nil {
				fmt.Fprintf(os.Stderr, "docgen: warning: failed to capture app help for %s: %v\n", relDir, usageErr)
				captureErr = usageErr.Error()
			}
			usage = captured
		}
		help := parseCommandHelp(strings.TrimSpace(usage))

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    "apps",
			BodyHTML:   renderAppReferenceBody(title, relDir, docText, sourceFile, previewPath, help, usesFlags, captureErr),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func appDisplayName(root, relDir string) (string, error) {
	manifestPath := filepath.Join(root, relDir, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	var manifest appManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return "", err
	}
	return strings.TrimSpace(manifest.Name), nil
}

func appPreviewPath(root, relDir string) string {
	name := strings.TrimSpace(filepath.Base(relDir))
	if name == "" {
		return ""
	}
	sourcePath := filepath.Join(root, relDir, "preview.png")
	if _, err := os.Stat(sourcePath); err != nil {
		return ""
	}
	return "assets/apps/" + name + "/preview.png"
}

func renderCommandDocs(root string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, "cmd")
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		docText, sourceFile, readErr := readProgramDoc(root, relDir, []string{"doc.go", "docs.go"})
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		usage := ""
		captureErr := ""
		usesFlags, detectErr := commandUsesFlagPackage(filepath.Join(root, relDir))
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to inspect command flags for %s: %v\n", relDir, detectErr)
			usesFlags = true
		}
		if usesFlags {
			captured, usageErr := commandUsageText(root, relDir)
			if usageErr != nil {
				fmt.Fprintf(os.Stderr, "docgen: warning: failed to capture flag.Usage for %s: %v\n", relDir, usageErr)
				captureErr = usageErr.Error()
			}
			usage = captured
		}
		usage = strings.TrimSpace(usage)
		parsed := parseCommandHelp(usage)

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    "cmd",
			BodyHTML:   renderCommandReferenceBody(title, relDir, docText, sourceFile, parsed, usesFlags, captureErr),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func commandUsesFlagPackage(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			return false, readErr
		}
		source := string(content)
		if strings.Contains(source, `"flag"`) || strings.Contains(source, "flag.") {
			return true, nil
		}
	}
	return false, nil
}

func renderAPIJSONDocs(root string) ([]apiDoc, error) {
	apiRoot := filepath.Join(root, "api")
	st, err := os.Stat(apiRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	var files []string
	if err := filepath.WalkDir(apiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "api.json") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)

	used := map[string]int{}
	out := make([]apiDoc, 0, len(files))
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}

		var spec apiSchema
		if unmarshalErr := json.Unmarshal(content, &spec); unmarshalErr != nil {
			return nil, fmt.Errorf("%s: %w", path, unmarshalErr)
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil, relErr
		}
		rel = filepath.ToSlash(rel)
		shortPath := strings.TrimSuffix(rel, "/api.json")
		shortPath = strings.TrimSuffix(shortPath, ".json")
		shortPath = filepath.ToSlash(strings.TrimSpace(shortPath))
		if shortPath == "" || shortPath == "." {
			continue
		}

		title := filepath.Base(shortPath)
		importPath := shortPath
		if name := strings.TrimSpace(spec.Service.Name); name != "" {
			importPath = name
		}
		if pkg := strings.TrimSpace(spec.Service.Package); pkg != "" {
			title = pkg
		}

		baseName := "api-" + slugify(shortPath)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: importPath,
			Title:      title,
			ShortPath:  shortPath,
			Section:    "api",
			BodyHTML:   renderAPIJSONBody(rel, spec),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func renderPkgDocs(root, modulePath string) ([]apiDoc, error) {
	packages, err := collectGoPackages(root, modulePath, "pkg", false)
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(packages))

	for _, pkg := range packages {
		bodyHTML, err := renderPackageBody(pkg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: skipping %s: %v\n", pkg.ImportPath, err)
			continue
		}

		baseName := "api-" + slugify(pkg.ImportPath)
		htmlName := uniqueHTMLName(baseName, used)
		shortPath := shortenImportPath(modulePath, pkg.ImportPath)
		out = append(out, apiDoc{
			ImportPath: pkg.ImportPath,
			Title:      shortPath,
			ShortPath:  shortPath,
			Section:    "pkg",
			BodyHTML:   bodyHTML,
			HTMLFile:   htmlName,
		})
	}
	return out, nil
}

func readProgramDoc(root, relDir string, preferredDocFiles []string) (docText, sourceFile string, err error) {
	for _, name := range preferredDocFiles {
		candidate := filepath.Join(root, relDir, name)
		st, statErr := os.Stat(candidate)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return "", "", statErr
		}
		if st.IsDir() {
			continue
		}

		docText, parseErr := packageDocFromFile(candidate)
		if parseErr != nil {
			return "", "", parseErr
		}
		docText = strings.TrimSpace(docText)
		if docText == "" {
			continue
		}
		rel, relErr := filepath.Rel(root, candidate)
		if relErr != nil {
			return docText, "", nil
		}
		return docText, filepath.ToSlash(rel), nil
	}

	docText, parseErr := packageDocForDir(filepath.Join(root, relDir), relDir)
	if parseErr != nil {
		return "", "", parseErr
	}
	return strings.TrimSpace(docText), "", nil
}

func packageDocFromFile(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if file == nil || file.Doc == nil {
		return "", nil
	}
	return strings.TrimSpace(file.Doc.Text()), nil
}

func packageDocForDir(dir, importPath string) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if len(pkgs) == 0 {
		return "", nil
	}

	var parsedPkg *ast.Package
	for _, p := range pkgs {
		parsedPkg = p
		break
	}
	if parsedPkg == nil {
		return "", nil
	}
	pkgDoc := doc.New(parsedPkg, importPath, 0)
	if pkgDoc == nil {
		return "", nil
	}
	return strings.TrimSpace(pkgDoc.Doc), nil
}

func commandUsageText(root, relDir string) (string, error) {
	tempDir, err := os.MkdirTemp("", "docgen-cmd-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	binPath := filepath.Join(tempDir, slugify(relDir))
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, "go", "build", "-o", binPath, "./"+relDir)
	buildCmd.Dir = root
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildCtx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(string(buildOut)), fmt.Errorf("timeout while building command")
	}
	if buildErr != nil {
		return strings.TrimSpace(string(buildOut)), buildErr
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer runCancel()

	runCmd := exec.CommandContext(runCtx, binPath, "-h")
	runCmd.Dir = root
	out, runErr := runCmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if runCtx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("timeout while capturing usage")
	}
	if runErr != nil && text == "" {
		return "", runErr
	}
	return text, nil
}

func parseCommandHelp(raw string) commandHelpDoc {
	doc := commandHelpDoc{
		Raw: strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n")),
	}
	if doc.Raw == "" {
		return doc
	}

	lines := strings.Split(doc.Raw, "\n")
	titleIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			titleIdx = i
			break
		}
	}
	if titleIdx < 0 {
		return doc
	}

	doc.Title = strings.TrimSpace(lines[titleIdx])
	if parts := strings.SplitN(doc.Title, " - ", 2); len(parts) == 2 {
		doc.Command = strings.TrimSpace(parts[0])
		doc.Synopsis = strings.TrimSpace(parts[1])
	}

	type sectionRef struct {
		key   string
		start int
		end   int
	}

	sections := make([]sectionRef, 0, 4)
	for i := titleIdx + 1; i < len(lines); i++ {
		if key := normalizeCommandHelpHeading(strings.TrimSpace(lines[i])); key != "" {
			sections = append(sections, sectionRef{
				key:   key,
				start: i,
			})
		}
	}
	for i := range sections {
		if i+1 < len(sections) {
			sections[i].end = sections[i+1].start
		} else {
			sections[i].end = len(lines)
		}
	}

	firstSection := len(lines)
	if len(sections) > 0 {
		firstSection = sections[0].start
	}
	for _, line := range lines[titleIdx+1 : firstSection] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		doc.Summary = append(doc.Summary, trimmed)
	}

	sectionStart := map[string]int{}
	sectionEnd := map[string]int{}
	for _, section := range sections {
		body := lines[section.start+1 : section.end]
		sectionStart[section.key] = section.start
		sectionEnd[section.key] = section.end
		switch section.key {
		case "usage":
			doc.Usage = parsePlainLines(body)
		case "subcommands":
			subcommandLines, flagLines := splitSubcommandAndFlagBlock(body)
			doc.Subcommands = parseCommandRows(subcommandLines)
			if len(doc.Flags) == 0 && len(flagLines) > 0 {
				doc.Flags = parseCommandFlags(flagLines)
			}
		case "exit_codes":
			doc.ExitCodes = parseCommandRows(body)
		case "flags":
			doc.Flags = parseCommandFlags(body)
		}
	}

	if len(doc.Flags) == 0 {
		start := titleIdx + 1
		if end, ok := sectionEnd["subcommands"]; ok {
			start = end
		} else if end, ok := sectionEnd["usage"]; ok {
			start = end
		}
		end := len(lines)
		if flagStart, ok := sectionStart["exit_codes"]; ok {
			end = flagStart
		}
		if start < end {
			doc.Flags = parseCommandFlags(lines[start:end])
		}
	}

	if len(doc.Summary) == 0 && doc.Synopsis != "" {
		doc.Summary = append(doc.Summary, doc.Synopsis)
	}

	return doc
}

func normalizeCommandHelpHeading(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasSuffix(line, ":") {
		return ""
	}
	line = strings.TrimSuffix(line, ":")
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "usage":
		return "usage"
	case "subcommands":
		return "subcommands"
	case "flags", "options":
		return "flags"
	case "exit codes", "exit code":
		return "exit_codes"
	default:
		return ""
	}
}

func parsePlainLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func parseCommandRows(lines []string) []commandHelpRow {
	out := make([]commandHelpRow, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if commandFlagLineRegex.MatchString(line) {
			continue
		}
		if trimmed == "(none)" {
			out = append(out, commandHelpRow{Name: "(none)", Description: ""})
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		desc := strings.TrimSpace(trimmed[len(name):])
		if desc == "" && len(fields) > 1 {
			desc = strings.Join(fields[1:], " ")
		}
		out = append(out, commandHelpRow{
			Name:        name,
			Description: strings.TrimSpace(desc),
		})
	}
	return out
}

func splitSubcommandAndFlagBlock(lines []string) (subcommands []string, flags []string) {
	for i, line := range lines {
		if commandFlagLineRegex.MatchString(line) {
			return lines[:i], lines[i:]
		}
	}
	return lines, nil
}

func parseCommandFlags(lines []string) []commandHelpFlag {
	out := make([]commandHelpFlag, 0, 8)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		match := commandFlagLineRegex.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}

		flagName := "-" + strings.TrimSpace(match[1])
		flagType := strings.TrimSpace(match[2])
		descriptionLines := make([]string, 0, 2)
		for i+1 < len(lines) {
			next := lines[i+1]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" {
				i++
				if len(descriptionLines) > 0 {
					break
				}
				continue
			}
			if commandFlagLineRegex.MatchString(next) || normalizeCommandHelpHeading(nextTrimmed) != "" {
				break
			}
			descriptionLines = append(descriptionLines, nextTrimmed)
			i++
		}

		description := strings.TrimSpace(strings.Join(descriptionLines, " "))
		if description == "" {
			description = "-"
		}
		if flagType == "" {
			flagType = "-"
		}

		out = append(out, commandHelpFlag{
			Name:        flagName,
			Type:        flagType,
			Description: description,
		})
	}

	return out
}

func executeDocTemplate(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := docTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func renderDocTemplate(name string, data any) template.HTML {
	out, err := executeDocTemplate(name, data)
	if err != nil {
		return template.HTML(`<p class="doc-ref-callout">Template render error: ` + template.HTMLEscapeString(err.Error()) + `</p>`)
	}
	return out
}

func renderProgramReferenceBody(section, title, relPath, docText, sourceFile string) template.HTML {
	var md strings.Builder
	kind := "Program"
	switch section {
	case "apps":
		kind = "Application"
	case "services":
		kind = "Service"
	}

	docText = strings.TrimSpace(docText)
	if docText != "" {
		md.WriteString("## Overview\n\n")
		md.WriteString(docText)
		md.WriteString("\n\n")
	} else {
		md.WriteString("## Overview\n\n")
		md.WriteString("Documentation not available yet. Add package comments in `doc.go` or `docs.go`.\n\n")
	}

	md.WriteString("## Reference\n\n")
	md.WriteString("| Field | Value |\n")
	md.WriteString("| --- | --- |\n")
	md.WriteString("| Type | " + markdownCell(kind) + " |\n")
	md.WriteString("| Path | " + markdownInlineCode(relPath) + " |\n")
	if sourceFile != "" {
		md.WriteString("| Doc Source | " + markdownInlineCode(sourceFile) + " |\n")
	} else {
		md.WriteString("| Doc Source | package comments |\n")
	}

	return renderSectionMarkdownBody(title, relPath, md.String())
}

func renderCommandReferenceBody(title, relPath, docText, sourceFile string, help commandHelpDoc, usesFlags bool, captureErr string) template.HTML {
	docText = strings.TrimSpace(docText)
	commandName := strings.TrimSpace(help.Command)
	if commandName == "" {
		commandName = filepath.Base(relPath)
	}

	lead := strings.TrimSpace(help.Synopsis)
	if lead == "" && len(help.Summary) > 0 {
		lead = strings.TrimSpace(help.Summary[0])
	}
	if lead == "" {
		lead = "Command reference"
	}

	usagePrimary := ""
	if len(help.Usage) > 0 {
		usagePrimary = strings.TrimSpace(help.Usage[0])
	}
	if usagePrimary == "" {
		usagePrimary = commandName
	}

	docSource := "package comments"
	if sourceFile != "" {
		docSource = sourceFile
	}

	helpSource := "command does not use `flag` package"
	if usesFlags {
		helpSource = "flag.Usage via -h"
	}

	usageLines := filterUsageLines(help.Usage)
	subcommands := filterCommandRows(help.Subcommands)
	flags := normalizeCommandFlags(help.Flags)
	exitCodes := filterExitCodeRows(help.ExitCodes)

	overview := template.HTML("")
	if docText != "" {
		overview = renderMarkdown(docText)
	}

	data := commandReferenceTemplateData{
		Title:           title,
		RelPath:         relPath,
		Lead:            lead,
		UsagePrimary:    usagePrimary,
		CommandName:     commandName,
		HelpSource:      helpSource,
		DocSource:       docSource,
		OverviewHTML:    overview,
		Usage:           usageLines,
		Subcommands:     subcommands,
		Flags:           flags,
		ExitCodes:       exitCodes,
		CaptureErr:      strings.TrimSpace(captureErr),
		NoFlagsAndNoDoc: !usesFlags && docText == "",
		ShowRawHelp:     len(usageLines) == 0 && len(subcommands) == 0 && len(flags) == 0 && strings.TrimSpace(help.Raw) != "",
		RawHelp:         strings.TrimSpace(help.Raw),
	}
	return renderDocTemplate("command_reference", data)
}

func renderAppReferenceBody(title, relPath, docText, sourceFile, previewPath string, help commandHelpDoc, usesFlags bool, captureErr string) template.HTML {
	docText = strings.TrimSpace(docText)
	if docText == "" {
		docText = "Documentation not available yet. Add package comments in `doc.go` or `docs.go`."
	}
	docSource := "package comments"
	if sourceFile != "" {
		docSource = sourceFile
	}

	lead := firstLine(docText)
	if lead == "" {
		lead = "Application reference"
	}

	flags := normalizeCommandFlags(help.Flags)
	subcommands := filterCommandRows(help.Subcommands)
	flagsMessage := "This app does not expose `flag.Usage` output."
	if usesFlags {
		flagsMessage = "No flags documented by app help output."
	}

	data := appReferenceTemplateData{
		Title:        title,
		RelPath:      relPath,
		Lead:         lead,
		DocSource:    docSource,
		PreviewPath:  strings.TrimSpace(previewPath),
		Description:  renderMarkdown(docText),
		Flags:        flags,
		Subcommands:  subcommands,
		FlagsMessage: flagsMessage,
		CaptureErr:   strings.TrimSpace(captureErr),
	}
	return renderDocTemplate("app_reference", data)
}

func firstLine(value string) string {
	for line := range strings.SplitSeq(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "#*- ")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func countCommandRows(rows []commandHelpRow) int {
	count := 0
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" || row.Name == "(none)" {
			continue
		}
		count++
	}
	return count
}

func filterUsageLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func filterCommandRows(rows []commandHelpRow) []commandHelpRow {
	out := make([]commandHelpRow, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" || name == "(none)" {
			continue
		}
		description := strings.TrimSpace(row.Description)
		if description == "" {
			description = "-"
		}
		out = append(out, commandHelpRow{
			Name:        name,
			Description: description,
		})
	}
	return out
}

func normalizeCommandFlags(flags []commandHelpFlag) []commandHelpFlag {
	out := make([]commandHelpFlag, 0, len(flags))
	for _, flag := range flags {
		name := strings.TrimSpace(flag.Name)
		if name == "" {
			continue
		}
		flagType := defaultText(flag.Type, "-")
		description := defaultText(flag.Description, "-")
		out = append(out, commandHelpFlag{
			Name:        name,
			Type:        flagType,
			Description: description,
		})
	}
	return out
}

func filterExitCodeRows(rows []commandHelpRow) []commandHelpRow {
	out := make([]commandHelpRow, 0, len(rows))
	for _, row := range rows {
		code := strings.TrimSpace(row.Name)
		if code == "" {
			continue
		}
		meaning := defaultText(row.Description, "-")
		out = append(out, commandHelpRow{
			Name:        code,
			Description: meaning,
		})
	}
	return out
}

func renderMarkdownCards(source string) template.HTML {
	return renderDocTemplate("markdown_cards", markdownCardsTemplateData{
		Sections: splitMarkdownSections(source),
	})
}

func splitMarkdownSections(source string) []markdownSection {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")

	currentTitle := "Overview"
	body := make([]string, 0, 32)
	out := make([]markdownSection, 0, 8)
	seenH2 := false

	flush := func() {
		content := strings.TrimSpace(strings.Join(body, "\n"))
		if content == "" {
			body = body[:0]
			return
		}
		out = append(out, markdownSection{
			Title:   strings.TrimSpace(currentTitle),
			Content: content,
		})
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			// Top heading is already rendered separately by docgen templates.
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			currentTitle = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if currentTitle == "" {
				currentTitle = "Section"
			}
			seenH2 = true
			continue
		}
		body = append(body, line)
	}
	flush()

	if len(out) == 0 {
		content := strings.TrimSpace(source)
		if content == "" {
			return nil
		}
		return []markdownSection{{
			Title:   "Overview",
			Content: content,
		}}
	}

	if !seenH2 && len(out) == 1 && strings.TrimSpace(out[0].Title) == "" {
		out[0].Title = "Overview"
	}
	return out
}

func renderSectionMarkdownBody(title, relPath, markdown string) template.HTML {
	return renderDocTemplate("section_markdown", sectionMarkdownTemplateData{
		Title:    title,
		RelPath:  relPath,
		Markdown: markdown,
	})
}

func renderAPIJSONBody(relPath string, spec apiSchema) template.HTML {
	title := strings.TrimSpace(spec.Service.Name)
	if title == "" {
		title = filepath.Base(filepath.Dir(relPath))
	}

	serviceDescription := strings.TrimSpace(spec.Service.Description)
	lead := serviceDescription
	if lead == "" {
		lead = "API reference for " + title
	}

	return renderDocTemplate("api_schema", apiSchemaTemplateData{
		Title:   title,
		RelPath: relPath,
		Lead:    lead,
		Spec:    spec,
	})
}

func methodPayloadType(method apiSchemaMethod) string {
	if value := strings.TrimSpace(method.RequestType); value != "" {
		return value
	}
	return strings.TrimSpace(method.PayloadType)
}

func responseType(method apiSchemaMethod) string {
	if method.OneWay {
		return "none (one-way)"
	}
	value := strings.TrimSpace(method.ResponseType)
	if value == "" {
		return "none"
	}
	return value
}

func requestDescription(method apiSchemaMethod) string {
	return defaultText(method.RequestDescription, "No input description.")
}

func responseDescription(method apiSchemaMethod) string {
	value := strings.TrimSpace(method.ResponseDescription)
	if value != "" {
		return value
	}
	if method.OneWay {
		return "No response payload for one-way request."
	}
	return "No output description."
}

func payloadDescription(method apiSchemaMethod) string {
	return defaultText(method.RequestDescription, "No payload description.")
}

func defaultText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func markdownInlineCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "`", "\\`")
	return "`" + value + "`"
}

func markdownCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func renderProjectBody(title, relPath, source string) template.HTML {
	return renderDocTemplate("project_page", projectPageTemplateData{
		Title:     title,
		RelPath:   relPath,
		CardsHTML: renderMarkdownCards(source),
	})
}

func renderAPIBody(d apiDoc) template.HTML {
	return d.BodyHTML
}

func renderPackageBody(pkg apiPackage) (template.HTML, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkg.Dir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if len(pkgs) == 0 {
		return "", errors.New("no non-test go files")
	}

	var parsedPkg *ast.Package
	for _, p := range pkgs {
		parsedPkg = p
		break
	}
	if parsedPkg == nil {
		return "", errors.New("unable to parse package")
	}

	pkgDoc := doc.New(parsedPkg, pkg.ImportPath, 0)
	if pkgDoc == nil {
		return "", errors.New("unable to build package documentation")
	}

	var b strings.Builder
	b.WriteString(`<section class="doc-api doc-go-shell">`)
	b.WriteString(`<header class="doc-ref-hero">`)
	b.WriteString("<h1>" + template.HTMLEscapeString(pkg.ImportPath) + "</h1>\n")
	b.WriteString(`<p class="muted mono doc-path">package ` + template.HTMLEscapeString(pkgDoc.Name) + "</p>\n")
	b.WriteString(`</header>`)

	var overview strings.Builder
	if strings.TrimSpace(pkgDoc.Doc) != "" {
		overview.WriteString(goDocToMarkdown(strings.TrimSpace(pkgDoc.Doc)))
		overview.WriteString("\n\n")
	} else {
		overview.WriteString("No package-level documentation is provided.\n\n")
	}
	overview.WriteString("| Export Group | Count |\n")
	overview.WriteString("| --- | --- |\n")
	overview.WriteString(fmt.Sprintf("| Constants | %d |\n", len(pkgDoc.Consts)))
	overview.WriteString(fmt.Sprintf("| Variables | %d |\n", len(pkgDoc.Vars)))
	overview.WriteString(fmt.Sprintf("| Functions | %d |\n", len(pkgDoc.Funcs)))
	overview.WriteString(fmt.Sprintf("| Types | %d |\n", len(pkgDoc.Types)))

	b.WriteString(`<section class="doc-section-card doc-go-group">`)
	b.WriteString("<h2>Overview</h2>")
	b.WriteString(`<section class="doc-markdown">`)
	b.WriteString(string(renderMarkdown(overview.String())))
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)

	appendValueSection(&b, "Constants", pkgDoc.Consts, fset)
	appendValueSection(&b, "Variables", pkgDoc.Vars, fset)
	appendFuncSection(&b, "Functions", pkgDoc.Funcs, fset)
	appendTypeSection(&b, pkgDoc.Types, fset)
	b.WriteString("</section>")
	return template.HTML(b.String()), nil
}

func appendValueSection(b *strings.Builder, heading string, values []*doc.Value, fset *token.FileSet) {
	if len(values) == 0 {
		return
	}

	b.WriteString(`<section class="doc-section-card doc-go-group">`)
	b.WriteString("<h2>" + template.HTMLEscapeString(heading) + "</h2>")
	b.WriteString(`<div class="api-card-list">`)
	for _, value := range values {
		name := strings.TrimSpace(strings.Join(value.Names, ", "))
		appendGoDeclCard(b, "doc-go-item", name, formattedDecl(fset, value.Decl), value.Doc)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func appendFuncSection(b *strings.Builder, heading string, funcs []*doc.Func, fset *token.FileSet) {
	if len(funcs) == 0 {
		return
	}

	b.WriteString(`<section class="doc-section-card doc-go-group">`)
	b.WriteString("<h2>" + template.HTMLEscapeString(heading) + "</h2>")
	b.WriteString(`<div class="api-card-list">`)
	for _, fn := range funcs {
		appendGoDeclCard(b, "doc-go-item", strings.TrimSpace(fn.Name), formattedDecl(fset, fn.Decl), fn.Doc)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func appendTypeSection(b *strings.Builder, types []*doc.Type, fset *token.FileSet) {
	if len(types) == 0 {
		return
	}

	b.WriteString(`<section class="doc-section-card doc-go-group">`)
	b.WriteString("<h2>Types</h2>")
	b.WriteString(`<div class="api-card-list">`)
	for _, t := range types {
		b.WriteString(`<article class="api-item-card doc-go-item">`)
		if strings.TrimSpace(t.Name) != "" {
			b.WriteString("<h3><code>" + template.HTMLEscapeString(t.Name) + "</code></h3>")
		}
		if decl := formattedDecl(fset, t.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(t.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(goDocToMarkdown(t.Doc))))
			b.WriteString(`</section>`)
		}

		appendTypeValues(b, "Constants", t.Consts, fset)
		appendTypeValues(b, "Variables", t.Vars, fset)
		appendTypeFuncs(b, "Functions", t.Funcs, fset)
		appendTypeFuncs(b, "Methods", t.Methods, fset)

		b.WriteString(`</article>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func appendTypeValues(b *strings.Builder, heading string, values []*doc.Value, fset *token.FileSet) {
	if len(values) == 0 {
		return
	}

	b.WriteString(`<section class="doc-go-subgroup">`)
	b.WriteString("<h4>" + template.HTMLEscapeString(heading) + "</h4>")
	b.WriteString(`<div class="api-card-list doc-go-sublist">`)
	for _, value := range values {
		name := strings.TrimSpace(strings.Join(value.Names, ", "))
		appendGoDeclCard(b, "doc-go-subitem", name, formattedDecl(fset, value.Decl), value.Doc)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func appendTypeFuncs(b *strings.Builder, heading string, funcs []*doc.Func, fset *token.FileSet) {
	if len(funcs) == 0 {
		return
	}

	b.WriteString(`<section class="doc-go-subgroup">`)
	b.WriteString("<h4>" + template.HTMLEscapeString(heading) + "</h4>")
	b.WriteString(`<div class="api-card-list doc-go-sublist">`)
	for _, fn := range funcs {
		appendGoDeclCard(b, "doc-go-subitem", strings.TrimSpace(fn.Name), formattedDecl(fset, fn.Decl), fn.Doc)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func appendGoDeclCard(b *strings.Builder, className, symbol, decl, docText string) {
	if strings.TrimSpace(className) == "" {
		className = "doc-go-item"
	}

	b.WriteString(`<article class="api-item-card ` + className + `">`)

	symbol = strings.TrimSpace(symbol)
	if symbol != "" {
		b.WriteString(`<p class="doc-go-symbol"><code>` + template.HTMLEscapeString(symbol) + `</code></p>`)
	}

	if decl != "" {
		b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
	}

	if strings.TrimSpace(docText) != "" {
		b.WriteString(`<section class="doc-markdown">`)
		b.WriteString(string(renderMarkdown(goDocToMarkdown(docText))))
		b.WriteString(`</section>`)
	}

	if decl == "" && strings.TrimSpace(docText) == "" {
		b.WriteString(`<p class="doc-ref-empty">No declaration details available.</p>`)
	}

	b.WriteString(`</article>`)
}

func formattedDecl(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func renderProjectIndexBody(projectDocs []markdownDoc) template.HTML {
	docs := make([]markdownDoc, 0, len(projectDocs))
	for _, d := range projectDocs {
		if d.RelPath == "README.md" {
			continue
		}
		docs = append(docs, d)
	}
	return renderDocTemplate("project_index", projectIndexTemplateData{Docs: docs})
}

func renderReadmeIndexBody(relPath, source string) template.HTML {
	return renderDocTemplate("readme_index", readmeIndexTemplateData{
		RelPath:   relPath,
		CardsHTML: renderMarkdownCards(source),
	})
}

type listFrame struct {
	tag    string
	indent int
}

func renderMarkdown(source string) template.HTML {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")

	var b strings.Builder
	var paragraph []string
	var listStack []listFrame
	inCode := false
	inHTMLBlock := false
	htmlBlockTag := ""

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		content := renderInline(strings.Join(paragraph, " "))
		b.WriteString("<p>" + content + "</p>\n")
		paragraph = paragraph[:0]
	}

	closeListsDeeper := func(targetIndent int) {
		for len(listStack) > 0 && listStack[len(listStack)-1].indent >= targetIndent {
			top := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			b.WriteString("</li>\n</" + top.tag + ">\n")
		}
	}

	flushList := func() {
		for len(listStack) > 0 {
			top := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			b.WriteString("</li>\n</" + top.tag + ">\n")
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)

		if inCode {
			if strings.HasPrefix(trimmed, "```") {
				b.WriteString("</code></pre>\n")
				inCode = false
				continue
			}
			b.WriteString(template.HTMLEscapeString(line) + "\n")
			continue
		}

		if inHTMLBlock {
			b.WriteString(line + "\n")
			if closesHTMLBlock(trimmed, htmlBlockTag) {
				inHTMLBlock = false
				htmlBlockTag = ""
			}
			continue
		}

		if trimmed == "" {
			flushParagraph()
			flushList()
			continue
		}

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			flushList()
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if lang == "" {
				b.WriteString("<pre><code>")
			} else {
				b.WriteString(`<pre><code class="language-` + template.HTMLEscapeString(lang) + `">`)
			}
			inCode = true
			continue
		}

		if tag, isStart, selfClosing := htmlBlockStartTag(trimmed); isStart {
			flushParagraph()
			flushList()
			b.WriteString(line + "\n")
			if !selfClosing && !isInlineHTMLTag(tag) && !closesHTMLBlock(trimmed, tag) {
				inHTMLBlock = true
				htmlBlockTag = tag
			}
			continue
		}

		if isRawHTMLLine(trimmed) {
			flushParagraph()
			flushList()
			b.WriteString(line + "\n")
			continue
		}

		if headingMatch := mdHeadingPattern.FindStringSubmatch(trimmed); len(headingMatch) == 3 {
			flushParagraph()
			flushList()
			level := min(max(len(headingMatch[1]), 1), 6)
			rawText := strings.TrimSpace(headingMatch[2])
			content := renderInline(rawText)
			anchor := slugifyHeading(rawText)
			b.WriteString(fmt.Sprintf(`<h%d id="%s"><a class="doc-anchor" href="#%s" aria-hidden="true">#</a>%s</h%d>`+"\n", level, anchor, anchor, content, level))
			continue
		}

		if isHorizontalRule(trimmed) {
			flushParagraph()
			flushList()
			b.WriteString("<hr />\n")
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			flushList()
			var quoteLines []string
			for ; i < len(lines); i++ {
				quoteLine := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(quoteLine, ">") {
					i--
					break
				}
				quoteLines = append(quoteLines, strings.TrimSpace(strings.TrimPrefix(quoteLine, ">")))
			}
			b.WriteString("<blockquote><p>" + renderInline(strings.Join(quoteLines, " ")) + "</p></blockquote>\n")
			continue
		}

		if strings.Contains(trimmed, "|") &&
			i+1 < len(lines) &&
			mdTableDividerPattern.MatchString(strings.TrimSpace(lines[i+1])) {
			flushParagraph()
			flushList()

			headers := parseTableCells(trimmed)
			if len(headers) == 0 {
				continue
			}

			b.WriteString("<table>\n<thead><tr>")
			for _, cell := range headers {
				b.WriteString("<th>" + renderInline(cell) + "</th>")
			}
			b.WriteString("</tr></thead>\n<tbody>\n")

			i++
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if next == "" || !strings.Contains(next, "|") {
					break
				}
				i++
				rowCells := parseTableCells(next)
				b.WriteString("<tr>")
				for c := range headers {
					cell := ""
					if c < len(rowCells) {
						cell = rowCells[c]
					}
					b.WriteString("<td>" + renderInline(cell) + "</td>")
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</tbody>\n</table>\n")
			continue
		}

		if listMatch := mdUnorderedPattern.FindStringSubmatch(line); len(listMatch) == 4 {
			flushParagraph()
			indent := len(listMatch[1])
			content := listMatch[3]
			if len(listStack) > 0 && indent > listStack[len(listStack)-1].indent {
				// deeper nesting: open new list inside current <li>
				b.WriteString("<ul>\n")
				listStack = append(listStack, listFrame{tag: "ul", indent: indent})
			} else if len(listStack) == 0 {
				b.WriteString("<ul>\n")
				listStack = append(listStack, listFrame{tag: "ul", indent: indent})
			} else {
				closeListsDeeper(indent)
				if len(listStack) == 0 || listStack[len(listStack)-1].tag != "ul" {
					b.WriteString("<ul>\n")
					listStack = append(listStack, listFrame{tag: "ul", indent: indent})
				} else {
					b.WriteString("</li>\n")
				}
			}
			b.WriteString("<li>" + renderInline(content))
			continue
		}

		if listMatch := mdOrderedPattern.FindStringSubmatch(line); len(listMatch) == 4 {
			flushParagraph()
			indent := len(listMatch[1])
			content := listMatch[3]
			if len(listStack) > 0 && indent > listStack[len(listStack)-1].indent {
				b.WriteString("<ol>\n")
				listStack = append(listStack, listFrame{tag: "ol", indent: indent})
			} else if len(listStack) == 0 {
				b.WriteString("<ol>\n")
				listStack = append(listStack, listFrame{tag: "ol", indent: indent})
			} else {
				closeListsDeeper(indent)
				if len(listStack) == 0 || listStack[len(listStack)-1].tag != "ol" {
					b.WriteString("<ol>\n")
					listStack = append(listStack, listFrame{tag: "ol", indent: indent})
				} else {
					b.WriteString("</li>\n")
				}
			}
			b.WriteString("<li>" + renderInline(content))
			continue
		}

		if len(listStack) > 0 {
			flushList()
		}
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	flushList()
	if inCode {
		b.WriteString("</code></pre>\n")
	}

	return template.HTML(b.String())
}

func renderInline(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	escaped := template.HTMLEscapeString(text)
	placeholders := make([]string, 0, 4)

	escaped = mdCodePattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdCodePattern.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		token := fmt.Sprintf("{{{CODE_%d}}}", len(placeholders))
		placeholders = append(placeholders, "<code>"+sub[1]+"</code>")
		return token
	})

	escaped = mdImagePattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdImagePattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		alt := strings.TrimSpace(sub[1])
		src := strings.TrimSpace(sub[2])
		if src == "" {
			return match
		}
		return `<img src="` + src + `" alt="` + alt + `" />`
	})

	escaped = mdLinkPattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdLinkPattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		label := strings.TrimSpace(sub[1])
		href := strings.TrimSpace(sub[2])
		if href == "" {
			return label
		}
		return `<a href="` + href + `">` + label + `</a>`
	})

	escaped = mdBoldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = mdItalicPattern.ReplaceAllString(escaped, "<em>$1</em>")
	escaped = mdStrikePattern.ReplaceAllString(escaped, "<del>$1</del>")

	for i, code := range placeholders {
		escaped = strings.ReplaceAll(escaped, fmt.Sprintf("{{{CODE_%d}}}", i), code)
	}
	return escaped
}

func parseTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func isRawHTMLLine(line string) bool {
	if line == "" || !strings.HasPrefix(line, "<") {
		return false
	}
	if strings.HasPrefix(line, "<!--") {
		return true
	}
	return strings.HasSuffix(line, ">")
}

func isHorizontalRule(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	line = strings.ReplaceAll(line, " ", "")
	if len(line) < 3 {
		return false
	}
	switch {
	case strings.Trim(line, "-") == "":
		return true
	case strings.Trim(line, "*") == "":
		return true
	case strings.Trim(line, "_") == "":
		return true
	default:
		return false
	}
}

func htmlBlockStartTag(line string) (tag string, isStart bool, selfClosing bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false, false
	}
	if strings.HasPrefix(line, "<!--") {
		return "!--", true, strings.Contains(line, "-->")
	}
	if strings.HasPrefix(line, "<?") || strings.HasPrefix(strings.ToLower(line), "<!doctype") {
		return "", true, true
	}
	if m := htmlSelfTagPattern.FindStringSubmatch(line); len(m) == 3 {
		return strings.ToLower(m[1]), true, true
	}
	if strings.HasPrefix(line, "</") {
		return "", false, false
	}
	if m := htmlOpenTagPattern.FindStringSubmatch(line); len(m) == 3 {
		return strings.ToLower(m[1]), true, false
	}
	return "", false, false
}

func closesHTMLBlock(line, tag string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if tag == "!--" {
		return strings.Contains(line, "-->")
	}
	if strings.Contains(strings.ToLower(line), "</"+strings.ToLower(tag)+">") {
		return true
	}
	if m := htmlCloseTagPattern.FindStringSubmatch(line); len(m) == 2 {
		return strings.EqualFold(m[1], tag)
	}
	return false
}

func isInlineHTMLTag(tag string) bool {
	switch strings.ToLower(tag) {
	case "a", "abbr", "b", "br", "code", "em", "i", "img", "kbd", "mark", "q", "s", "small", "span", "strong", "sub", "sup", "time", "u", "var":
		return true
	default:
		return false
	}
}

// goDocToMarkdown converts Go doc comment text (as returned by go/doc) to
// markdown suitable for renderMarkdown. It handles two things:
//   - Tab-indented lines (Go's code-block convention) → fenced ```go blocks
//   - Go 1.19+ heading syntax (# Heading) → bumped level (## / ###) so the
//     page h1 is not duplicated
func goDocToMarkdown(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var out strings.Builder
	inCode := false

	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if !inCode {
				out.WriteString("```go\n")
				inCode = true
			}
			out.WriteString(strings.TrimPrefix(line, "\t") + "\n")
		} else {
			if inCode {
				out.WriteString("```\n")
				inCode = false
			}
			switch {
			case strings.HasPrefix(line, "# "):
				out.WriteString("## " + strings.TrimPrefix(line, "# ") + "\n")
			case strings.HasPrefix(line, "## "):
				out.WriteString("### " + strings.TrimPrefix(line, "## ") + "\n")
			default:
				out.WriteString(line + "\n")
			}
		}
	}
	if inCode {
		out.WriteString("```\n")
	}
	return out.String()
}

func slugifyHeading(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	prevDash := true
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func rewriteDocLinks(source, currentRel string, markdownMap map[string]string) string {
	rewritten := markdownLinkPattern.ReplaceAllStringFunc(source, func(match string) string {
		sub := markdownLinkPattern.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		targetPath, suffix := splitLinkTarget(sub[2])
		newPath := rewriteLinkTarget(targetPath, currentRel, markdownMap)
		return sub[1] + newPath + suffix + sub[3]
	})

	rewritten = htmlAttrLinkPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		sub := htmlAttrLinkPattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		newPath := rewriteLinkTarget(sub[2], currentRel, markdownMap)
		return sub[1] + `="` + newPath + `"`
	})

	return rewritten
}

func splitLinkTarget(raw string) (targetPath, suffix string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	for i, r := range raw {
		if unicode.IsSpace(r) {
			return raw[:i], raw[i:]
		}
	}
	return raw, ""
}

func rewriteLinkTarget(target, currentRel string, markdownMap map[string]string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return target
	}
	if strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#") {
		return target
	}

	wrapped := strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">")
	if wrapped {
		target = strings.TrimPrefix(strings.TrimSuffix(target, ">"), "<")
	}

	fragment := ""
	if idx := strings.Index(target, "#"); idx >= 0 {
		fragment = target[idx:]
		target = target[:idx]
	}

	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "/")

	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(currentRel), target)))

	if newPath, ok := markdownMap[resolved]; ok {
		out := newPath + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	if after, ok := strings.CutPrefix(resolved, "docs/assets/"); ok {
		out := "assets/" + after + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	if resolved == "data/icons/logo/logo.png" {
		out := "assets/logo.png" + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	out := target + fragment
	if wrapped {
		return "<" + out + ">"
	}
	return out
}

func buildHead(title, description string) template.HTML {
	head := fallbackHead

	head = scriptTagPattern.ReplaceAllString(head, "")
	head = stylesheetLinkPattern.ReplaceAllString(head, "")

	titleTag := "<title>" + template.HTMLEscapeString(title) + "</title>"
	if titleTagPattern.MatchString(head) {
		head = titleTagPattern.ReplaceAllString(head, titleTag)
	} else {
		head = insertBeforeHeadClose(head, titleTag)
	}

	descTag := `<meta name="description" content="` + template.HTMLEscapeString(description) + `" />`
	if descriptionMetaTag.MatchString(head) {
		head = descriptionMetaTag.ReplaceAllString(head, descTag)
	} else {
		head = insertBeforeHeadClose(head, descTag)
	}

	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="styles.css" />`)
	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="docgen.css" />`)
	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css" media="(prefers-color-scheme: light)" />`)
	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css" media="(prefers-color-scheme: dark)" />`)
	head = insertBeforeHeadClose(head, `<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js" defer></script>`)
	return template.HTML(head)
}

func insertBeforeHeadClose(head, insert string) string {
	lower := strings.ToLower(head)
	idx := strings.LastIndex(lower, "</head>")
	if idx < 0 {
		return head + "\n" + insert
	}
	return head[:idx] + "  " + insert + "\n" + head[idx:]
}

func parseDocsIndex(projectDocs []markdownDoc) docsIndexMeta {
	meta := docsIndexMeta{
		Order:  map[string]int{},
		Titles: map[string]string{},
	}

	indexDoc, ok := findProjectDoc(projectDocs, "docs/index.md")
	if !ok {
		return meta
	}

	order := 0
	for line := range strings.SplitSeq(indexDoc.Source, "\n") {
		matches := mdLinkPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) != 3 {
				continue
			}
			title := strings.TrimSpace(match[1])
			target := strings.TrimSpace(match[2])
			targetPath, _ := splitLinkTarget(target)
			relPath, ok := resolveMarkdownRelPath(indexDoc.RelPath, targetPath)
			if !ok {
				continue
			}
			if strings.ToLower(filepath.Ext(relPath)) != ".md" {
				continue
			}
			if _, exists := meta.Order[relPath]; !exists {
				meta.Order[relPath] = order
				order++
			}
			if title != "" {
				meta.Titles[relPath] = title
			}
		}
	}

	return meta
}

func buildSidebar(projectDocs []markdownDoc, apiDocs []apiDoc, docsIndex docsIndexMeta, homeRelPath, active string) []navSection {
	var sections []navSection

	if docsSection := buildDocsSection(projectDocs, docsIndex, homeRelPath, active); len(docsSection.Groups) > 0 {
		sections = append(sections, docsSection)
	}

	for _, spec := range []struct {
		title   string
		section string
	}{
		{title: "Apps", section: "apps"},
		{title: "Commands", section: "cmd"},
		{title: "Services", section: "services"},
		{title: "API", section: "api"},
		{title: "Packages", section: "pkg"},
	} {
		navSection := buildGoSection(spec.title, spec.section, apiDocs, active)
		if len(navSection.Groups) > 0 {
			sections = append(sections, navSection)
		}
	}

	return sections
}

func buildDocsSection(projectDocs []markdownDoc, docsIndex docsIndexMeta, homeRelPath, active string) navSection {
	type rankedEntry struct {
		entry navEntry
		order int
	}

	groups := map[string][]rankedEntry{}
	for i, d := range projectDocs {
		if d.RelPath == homeRelPath || d.RelPath == "README.md" {
			continue
		}
		if !strings.HasPrefix(d.RelPath, "docs/") {
			continue
		}

		title := strings.TrimSpace(d.Title)
		if override, ok := docsIndex.Titles[d.RelPath]; ok && strings.TrimSpace(override) != "" {
			title = strings.TrimSpace(override)
		}
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(d.RelPath), filepath.Ext(d.RelPath))
		}

		sortOrder := 100000 + i
		if order, ok := docsIndex.Order[d.RelPath]; ok {
			sortOrder = order
		}

		group := docsGroupName(d.RelPath)
		groups[group] = append(groups[group], rankedEntry{
			entry: navEntry{
				Title:  title,
				Href:   d.HTMLFile,
				Active: d.HTMLFile == active,
			},
			order: sortOrder,
		})
	}

	out := navSection{
		Title: "Docs",
		Groups: []navGroup{{
			Entries: []navEntry{{
				Title:  "Overview",
				Href:   "index.html",
				Active: active == "index.html",
			}},
		}},
	}

	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		if groupNames[i] == "General" {
			return true
		}
		if groupNames[j] == "General" {
			return false
		}
		return strings.ToLower(groupNames[i]) < strings.ToLower(groupNames[j])
	})

	for _, group := range groupNames {
		entries := groups[group]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].order != entries[j].order {
				return entries[i].order < entries[j].order
			}
			return strings.ToLower(entries[i].entry.Title) < strings.ToLower(entries[j].entry.Title)
		})

		groupNav := navGroup{Title: group}
		groupNav.Entries = make([]navEntry, 0, len(entries))
		for _, ranked := range entries {
			groupNav.Entries = append(groupNav.Entries, ranked.entry)
		}
		out.Groups = append(out.Groups, groupNav)
	}

	return out
}

func buildGoSection(title, section string, apiDocs []apiDoc, active string) navSection {
	groups := map[string][]navEntry{}

	for _, d := range apiDocs {
		if d.Section != section {
			continue
		}

		tail := strings.TrimPrefix(d.ShortPath, section+"/")
		if tail == d.ShortPath {
			tail = d.ShortPath
		}
		tail = strings.TrimSpace(tail)
		if tail == "" {
			continue
		}

		group := filepath.ToSlash(filepath.Dir(tail))
		if group == "." {
			group = ""
		}
		linkTitle := filepath.Base(tail)
		if linkTitle == "." || linkTitle == "/" || linkTitle == "" {
			linkTitle = tail
		}
		if section == "apps" && strings.TrimSpace(d.Title) != "" {
			linkTitle = strings.TrimSpace(d.Title)
		}

		groups[group] = append(groups[group], navEntry{
			Title:  linkTitle,
			Href:   d.HTMLFile,
			Active: d.HTMLFile == active,
		})
	}

	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		if groupNames[i] == "" {
			return true
		}
		if groupNames[j] == "" {
			return false
		}
		return strings.ToLower(groupNames[i]) < strings.ToLower(groupNames[j])
	})

	out := navSection{Title: title}
	for _, group := range groupNames {
		entries := groups[group]
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
		})
		groupTitle := group
		if groupTitle == "" {
			groupTitle = "Core"
		}
		out.Groups = append(out.Groups, navGroup{
			Title:   groupTitle,
			Entries: entries,
		})
	}
	return out
}

func docsGroupName(relPath string) string {
	if !strings.HasPrefix(relPath, "docs/") {
		return "General"
	}
	inside := strings.TrimPrefix(relPath, "docs/")
	dir := filepath.ToSlash(filepath.Dir(inside))
	if dir == "." || dir == "" {
		return "General"
	}
	return dir
}

func resolveMarkdownRelPath(currentRel, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	if strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#") {
		return "", false
	}

	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "/")

	rel := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(currentRel), target)))
	if rel == "." || rel == "" {
		return "", false
	}
	return rel, true
}

func renderPage(path string, data pageData) error {
	body, err := executeDocTemplate("page", data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0644)
}

func uniqueHTMLName(base string, used map[string]int) string {
	if used[base] == 0 {
		used[base] = 1
		return base + ".html"
	}
	used[base]++
	return fmt.Sprintf("%s-%d.html", base, used[base]-1)
}

func markdownTitle(content, fallback string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

func findProjectDoc(docs []markdownDoc, relPath string) (markdownDoc, bool) {
	for _, d := range docs {
		if d.RelPath == relPath {
			return d, true
		}
	}
	return markdownDoc{}, false
}

func packageNameForDir(dir string) (name string, hasGoFiles bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if strings.HasSuffix(filename, ".go") && !strings.HasSuffix(filename, "_test.go") {
			files = append(files, filepath.Join(dir, filename))
		}
	}
	if len(files) == 0 {
		return "", false, nil
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	for _, path := range files {
		file, parseErr := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if parseErr != nil {
			return "", true, parseErr
		}
		if file != nil && file.Name != nil {
			return file.Name.Name, true, nil
		}
	}
	return "", true, nil
}

func shortenImportPath(modulePath, importPath string) string {
	if modulePath == "" || !strings.HasPrefix(importPath, modulePath) {
		return importPath
	}
	short := strings.TrimPrefix(importPath, modulePath)
	short = strings.TrimPrefix(short, "/")
	if short == "" {
		return importPath
	}
	return short
}

func slugify(input string) string {
	input = strings.ToLower(input)
	var out strings.Builder
	lastDash := false

	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}

	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		return "doc"
	}
	return slug
}

func detectModulePath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			modulePath := strings.TrimSpace(after)
			if modulePath == "" {
				break
			}
			return modulePath, nil
		}
	}
	return "", fmt.Errorf("%s: module directive not found", goModPath)
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirIfExists(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return nil
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyAppPreviewAssets(root, outDir string) error {
	appsDir := filepath.Join(root, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}

		src := filepath.Join(appsDir, name, "preview.png")
		if _, statErr := os.Stat(src); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return statErr
		}

		dst := filepath.Join(outDir, "assets", "apps", name, "preview.png")
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	return nil
}

func copyOptionalFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return copyFile(src, dst)
}
