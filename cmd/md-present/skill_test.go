package main

import (
	"bytes"
	"os"
	"testing"
)

func TestRootSkillMatchesPluginSkill(t *testing.T) {
	rootSkill, err := os.ReadFile("../../SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	pluginSkill, err := os.ReadFile("../../skills/md-present/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rootSkill, pluginSkill) {
		t.Fatal("root SKILL.md and plugin skill differ")
	}
	codexSkill, err := os.ReadFile("../../plugins/md-present/skills/md-present/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rootSkill, codexSkill) {
		t.Fatal("root SKILL.md and Codex marketplace skill differ")
	}
}
