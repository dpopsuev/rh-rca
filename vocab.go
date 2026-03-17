package rca

import (
	"fmt"
	"strings"

	framework "github.com/dpopsuev/origami"
	"gopkg.in/yaml.v3"
)

type vocabFile struct {
	DefectTypes map[string]framework.VocabEntry `yaml:"defect_types"`
	Stages      map[string]framework.VocabEntry `yaml:"stages"`
	Metrics     map[string]framework.VocabEntry `yaml:"metrics"`
	Decisions   map[string]framework.VocabEntry `yaml:"decisions"`
	// Legacy alias: accepted on read, merged into Decisions.
	Heuristics map[string]framework.VocabEntry `yaml:"heuristics"`
}

// NewVocabulary builds and returns a fully populated RichMapVocabulary
// containing domain codes: defect types, circuit stages, metrics, and
// decisions. When data is nil an empty vocabulary is returned.
func NewVocabulary(data []byte) *framework.RichMapVocabulary {
	v := framework.NewRichMapVocabulary()
	if data == nil {
		return v
	}

	var f vocabFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		panic(fmt.Sprintf("vocabulary: parse YAML: %v", err))
	}

	backfillShort(f.DefectTypes)
	backfillShort(f.Stages)
	backfillShort(f.Metrics)
	backfillShort(f.Decisions)
	backfillShort(f.Heuristics)

	v.RegisterEntries(f.DefectTypes)
	v.RegisterEntries(f.Stages)
	v.RegisterEntries(deriveStageAliases(f.Stages))
	v.RegisterEntries(f.Metrics)
	v.RegisterEntries(f.Decisions)
	v.RegisterEntries(f.Heuristics)
	return v
}

// deriveStageAliases generates aliases like F0_RECALL from F0 -> "Recall".
func deriveStageAliases(stages map[string]framework.VocabEntry) map[string]framework.VocabEntry {
	aliases := make(map[string]framework.VocabEntry)
	for code, entry := range stages {
		if entry.Long == "" {
			continue
		}
		alias := code + "_" + strings.ToUpper(strings.ReplaceAll(entry.Long, " ", "_"))
		aliases[alias] = entry
	}
	return aliases
}

// backfillShort sets Short = key for entries loaded via shorthand (string value).
func backfillShort(m map[string]framework.VocabEntry) {
	for k, e := range m {
		if e.Short == "" {
			e.Short = k
			m[k] = e
		}
	}
}

// SourceIssueTag formats a source-provided issue type with a trust indicator.
func SourceIssueTag(v framework.Vocabulary, issueType string, autoAnalyzed bool) string {
	if issueType == "" {
		return ""
	}
	tag := "[human]"
	if autoAnalyzed {
		tag = "[auto]"
	}
	return v.Name(issueType) + " " + tag
}

// StagePath converts a slice of stage codes to a human-readable path.
func StagePath(v framework.Vocabulary, codes []string) string {
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = v.Name(c)
	}
	return strings.Join(names, " \u2192 ")
}

// ClusterKey humanizes a pipe-delimited cluster key.
func ClusterKey(v framework.Vocabulary, key string) string {
	parts := strings.Split(key, "|")
	for i, p := range parts {
		if name := v.Name(p); name != p {
			parts[i] = name
		}
	}
	return strings.Join(parts, " / ")
}

var (
	defaultVocab       = NewVocabulary(nil)
	defaultDefectTypes map[string]framework.VocabEntry
)

// InitVocab replaces the package-level vocabulary with one loaded from data.
func InitVocab(data []byte) {
	if data != nil {
		defaultVocab = NewVocabulary(data)
		var f vocabFile
		if err := yaml.Unmarshal(data, &f); err == nil {
			backfillShort(f.DefectTypes)
			defaultDefectTypes = f.DefectTypes
		}
	}
}

func vocabName(code string) string {
	return defaultVocab.Name(code)
}

func vocabNameWithCode(code string) string {
	return framework.NameWithCode(defaultVocab, code)
}

func vocabStagePath(codes []string) string {
	return StagePath(defaultVocab, codes)
}

func vocabSourceIssueTag(issueType string, autoAnalyzed bool) string {
	return SourceIssueTag(defaultVocab, issueType, autoAnalyzed)
}
